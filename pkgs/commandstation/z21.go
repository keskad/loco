package commandstation

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/sirupsen/logrus"
)

// NewZ21Roco constructor
func NewZ21Roco(netAddr string, netPort uint16) (*Z21Roco, error) {
	roco := Z21Roco{Timeout: time.Second * 10, wasPowerCutOff: false}
	return &roco, roco.connect(fmt.Sprintf("%s:%d", netAddr, netPort))
}

type Z21Roco struct {
	conn           net.Conn
	Timeout        time.Duration
	wasPowerCutOff bool
}

func (z *Z21Roco) connect(netAddr string) error {
	conn, err := net.Dial("udp", netAddr)
	if err != nil {
		return fmt.Errorf("UDP dial error while connecting to Roco Z21: %s", err)
	}
	z.conn = conn
	return nil
}

func (Z *Z21Roco) CleanUp() error {
	if Z.wasPowerCutOff {
		logrus.Debug("Restoring power on programming track")
		Z.buildTrackPowerOn()
	}
	return Z.conn.Close()
}

func (Z *Z21Roco) markBuildTrackPowerOff() {
	logrus.Debug("Marking programmng track as powered off")
	Z.wasPowerCutOff = true
}

func (z *Z21Roco) buildCVRequest(mode Mode, lcv LocoCV, isWriteRequest bool) ([]byte, error) {
	var err error
	var req []byte

	switch mode {
	case MainTrackMode:
		if isWriteRequest {
			req = z.buildPomWriteByte(lcv)
		} else {
			req = z.buildPomReadPacket(lcv)
		}
	case ProgrammingTrackMode:
		if isWriteRequest {
			req = z.buildProgWritePacket(lcv)
		} else {
			req = z.buildProgReadPacket(lcv.Cv)
		}
	default:
		return []byte{}, errors.New("unrecognized mode")
	}

	return req, err
}

func (z *Z21Roco) WriteCV(mode Mode, lcv LocoCV, options ...ctxOptions) error {
	ctx := RequestContext{timeout: z.Timeout, verify: false, retries: 2, settle: 200}
	applyMethodsToCtx(&ctx, options)

	req, err := z.buildCVRequest(mode, lcv, true)
	if err != nil {
		return fmt.Errorf("cannot build CV request in WriteCV: %s", err.Error())
	}

	// we need to restore the power later on
	if mode == ProgrammingTrackMode {
		defer z.markBuildTrackPowerOff()
	}

	logrus.Debug("Writing CV")
	if _, writeErr := z.conn.Write(req); err != nil {
		return fmt.Errorf("cannot write CV: %s", writeErr.Error())
	}

	if ctx.verify {
		logrus.Debug("Verifying written CV")
		time.Sleep(ctx.settle)
		res, readErr := z.readCVValue(mode, lcv, ctx.timeout, ctx.retries)
		if readErr != nil {
			return fmt.Errorf("cannot verify CV was written: %s", readErr.Error())
		}
		if res.value != byte(lcv.Cv.Value) {
			return fmt.Errorf("cannot write CV, the value differs after a write")
		}
	}

	return nil
}

// ReadCV reads a CV
func (z *Z21Roco) ReadCV(mode Mode, lcv LocoCV, options ...ctxOptions) (int, error) {
	ctx := RequestContext{timeout: z.Timeout, verify: false, retries: 2, settle: 200}
	applyMethodsToCtx(&ctx, options)

	// we need to restore the power later on
	if mode == ProgrammingTrackMode {
		defer z.markBuildTrackPowerOff()
	}

	res, readErr := z.readCVValue(mode, lcv, ctx.timeout, ctx.retries)
	if readErr != nil {
		return 0, fmt.Errorf("cannot read CV: %s", readErr.Error())
	}
	return int(res.value), nil
}

type cvResult struct {
	cv     uint16 // 0=CV1 (N+1)
	value  byte
	source string // LAN_X_CV_RESULT/NACK/NACK_SC
}

func (res *cvResult) Error() error {
	switch res.source {
	// ok, we return a correct result
	case "LAN_X_CV_RESULT":
		return nil
	// below are errors returned by Command Station, so the network is okay, but the error is on the protocol side / input data
	case "LAN_X_CV_NACK":
		return fmt.Errorf("missing RailCom acknowledgement (NACK_SC)")
	case "LAN_X_CV_NACK_SC":
		return fmt.Errorf("short circuit (LAN_X_CV_NACK_SC)")
	}
	return fmt.Errorf("unknown error (%s)", res.source)
}

func (z *Z21Roco) parseCVResponse(pkt []byte) (cvResult, bool) {
	if len(pkt) < 6 {
		return cvResult{}, false
	}
	dataLen := binary.LittleEndian.Uint16(pkt[0:2])
	header := binary.LittleEndian.Uint16(pkt[2:4])
	if header != 0x0040 || int(dataLen) != len(pkt) {
		return cvResult{}, false
	}

	// RESULT: 64 14 CV_MSB CV_LSB Value XOR
	if len(pkt) >= 10 && pkt[4] == 0x64 && pkt[5] == 0x14 {
		return cvResult{
			cv:     (uint16(pkt[6]) << 8) | uint16(pkt[7]),
			value:  pkt[8],
			source: "LAN_X_CV_RESULT",
		}, true
	}
	// NACKs
	if pkt[4] == 0x61 && pkt[5] == 0x13 {
		return cvResult{source: "LAN_X_CV_NACK"}, true
	}
	if pkt[4] == 0x61 && pkt[5] == 0x12 {
		return cvResult{source: "LAN_X_CV_NACK_SC"}, true
	}
	return cvResult{}, false
}

// Sends and waits for LAN_X_CV_* (read or write-result)
func (z *Z21Roco) sendAndAwait(conn net.Conn, req []byte, timeout time.Duration) (cvResult, error) {
	if _, err := conn.Write(req); err != nil {
		return cvResult{}, err
	}
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 1500)
	end := time.Now().Add(timeout)
	for time.Now().Before(end) {
		n, err := conn.Read(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				return cvResult{}, errors.New("response timeout")
			}
			return cvResult{}, err
		}
		if res, ok := z.parseCVResponse(buf[:n]); ok {
			return res, nil
		}
	}
	return cvResult{}, errors.New("no response or unrecognized response")
}

// readCVValue is reading the POM/PROG CV response
func (z *Z21Roco) readCVValue(mode Mode, lcv LocoCV, timeout time.Duration, retries int) (cvResult, error) {
	req, reqErr := z.buildCVRequest(mode, lcv, false)
	if reqErr != nil {
		return cvResult{}, fmt.Errorf("cannot build CV request: %s", reqErr)
	}

	var lastErr error
	for i := 0; i <= retries; i++ {
		res, err := z.sendAndAwait(z.conn, req, timeout)
		if err == nil {
			if responseErr := res.Error(); responseErr != nil {
				lastErr = fmt.Errorf("cannot read CV: %s", responseErr.Error())
				continue
			}

			return res, nil
		}
		lastErr = err
		time.Sleep(200 * time.Millisecond)
	}
	return cvResult{}, lastErr
}

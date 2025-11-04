package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// ===== Utils =====
func xorSum(b []byte) byte {
	var x byte
	for _, v := range b {
		x ^= v
	}
	return x
}

// ===== POM (PoM) =====
// Read: LAN_X_CV_POM_READ_BYTE (E6 30 … option 0xE4)
func buildPomReadPacket(addr uint16, cvWire uint16) []byte {
	const dataLen, header = 0x000C, 0x0040
	adrMSB := byte((addr >> 8) & 0x3F)
	if addr >= 128 {
		adrMSB |= 0xC0
	}
	adrLSB := byte(addr & 0xFF)
	db3 := byte(0xE4 | byte((cvWire>>8)&0x03)) // 111001MM
	db4 := byte(cvWire & 0xFF)
	x := []byte{0xE6, 0x30, adrMSB, adrLSB, db3, db4, 0x00}
	x = append(x, xorSum(x))
	buf := make([]byte, 0, 2+2+len(x))
	tmp := make([]byte, 2)
	binary.LittleEndian.PutUint16(tmp, dataLen)
	buf = append(buf, tmp...)
	binary.LittleEndian.PutUint16(tmp, header)
	buf = append(buf, tmp...)
	return append(buf, x...)
}

// Write BYTE: LAN_X_CV_POM_WRITE_BYTE (E6 30 … option 0xEC)
func buildPomWriteByte(addr uint16, cvWire uint16, value byte) []byte {
	const dataLen, header = 0x000C, 0x0040
	adrMSB := byte((addr >> 8) & 0x3F)
	if addr >= 128 {
		adrMSB |= 0xC0
	}
	adrLSB := byte(addr & 0xFF)
	db3 := byte(0xEC | byte((cvWire>>8)&0x03)) // 111011MM
	db4 := byte(cvWire & 0xFF)
	x := []byte{0xE6, 0x30, adrMSB, adrLSB, db3, db4, value}
	x = append(x, xorSum(x))
	buf := make([]byte, 0, 2+2+len(x))
	tmp := make([]byte, 2)
	binary.LittleEndian.PutUint16(tmp, dataLen)
	buf = append(buf, tmp...)
	binary.LittleEndian.PutUint16(tmp, header)
	buf = append(buf, tmp...)
	return append(buf, x...)
}

// ===== PROG (tor programujący / Direct Mode) =====
// Read: LAN_X_CV_READ (23 11)
func buildProgReadPacket(cvWire uint16) []byte {
	const dataLen, header = 0x0009, 0x0040
	x := []byte{0x23, 0x11, byte(cvWire >> 8), byte(cvWire & 0xFF)}
	x = append(x, xorSum(x))
	buf := make([]byte, 0, 2+2+len(x))
	tmp := make([]byte, 2)
	binary.LittleEndian.PutUint16(tmp, dataLen)
	buf = append(buf, tmp...)
	binary.LittleEndian.PutUint16(tmp, header)
	buf = append(buf, tmp...)
	return append(buf, x...)
}

// Write: LAN_X_CV_WRITE (24 12)
func buildProgWritePacket(cvWire uint16, value byte) []byte {
	const dataLen, header = 0x000A, 0x0040
	x := []byte{0x24, 0x12, byte(cvWire >> 8), byte(cvWire & 0xFF), value}
	x = append(x, xorSum(x))
	buf := make([]byte, 0, 2+2+len(x))
	tmp := make([]byte, 2)
	binary.LittleEndian.PutUint16(tmp, dataLen)
	buf = append(buf, tmp...)
	binary.LittleEndian.PutUint16(tmp, header)
	buf = append(buf, tmp...)
	return append(buf, x...)
}

// Track power ON (wyjście z programming mode)
func buildTrackPowerOn() []byte {
	const dataLen, header = 0x0007, 0x0040
	x := []byte{0x21, 0x81}
	x = append(x, xorSum(x))
	buf := make([]byte, 0, 2+2+len(x))
	tmp := make([]byte, 2)
	binary.LittleEndian.PutUint16(tmp, dataLen)
	buf = append(buf, tmp...)
	binary.LittleEndian.PutUint16(tmp, header)
	buf = append(buf, tmp...)
	return append(buf, x...)
}

// ===== Parsowanie wyników =====
type cvResult struct {
	cv     uint16 // 0=CV1 na drucie
	value  byte
	source string // LAN_X_CV_RESULT/NACK/NACK_SC
}

func parseCvResult(pkt []byte) (cvResult, bool) {
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

// Wysyła pojedyncze żądanie i czeka na LAN_X_CV_* (read or write-result)
func sendAndAwait(conn net.Conn, req []byte, timeout time.Duration) (cvResult, error) {
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
				return cvResult{}, errors.New("timeout na odpowiedź")
			}
			return cvResult{}, err
		}
		if res, ok := parseCvResult(buf[:n]); ok {
			return res, nil
		}
	}
	return cvResult{}, errors.New("brak odpowiedzi z wynikiem")
}

// Pomocniczy odczyt dla weryfikacji (tryb-świadomy)
func readForVerify(conn net.Conn, mode string, addr uint16, cvWire uint16, timeout time.Duration, retries int) (cvResult, error) {
	var req []byte
	if mode == "pom" {
		req = buildPomReadPacket(addr, cvWire)
	} else {
		req = buildProgReadPacket(cvWire)
	}
	var lastErr error
	for i := 0; i <= retries; i++ {
		res, err := sendAndAwait(conn, req, timeout)
		if err == nil {
			return res, nil
		}
		lastErr = err
		time.Sleep(200 * time.Millisecond)
	}
	return cvResult{}, lastErr
}

// ===== Main =====
func main() {
	var (
		z21Addr = flag.String("z21", "192.168.0.111:21105", "Adres IP:port Z21 (UDP)")
		mode    = flag.String("mode", "pom", "Tryb: pom | prog")
		op      = flag.String("op", "read", "Operacja: read | write")
		cvHuman = flag.Uint("cv", 1, "Numer CV (>=1)")
		loco    = flag.Uint("addr", 3, "Adres lokomotywy (dla -mode=pom)")
		value   = flag.Int("value", 0, "Wartość CV (0..255) dla -op=write")
		timeout = flag.Duration("timeout", 5*time.Second, "Timeout na odpowiedź")
		retries = flag.Int("retries", 2, "Liczba ponowień")

		verify = flag.Bool("verify", false, "Po zapisie: odczytaj i porównaj wartość CV")
		settle = flag.Duration("settle", 200*time.Millisecond, "Pauza po zapisie przed weryfikacją")
	)
	flag.Parse()

	m := strings.ToLower(*mode)
	o := strings.ToLower(*op)

	if *cvHuman < 1 {
		fmt.Fprintln(os.Stderr, "CV musi być >= 1.")
		os.Exit(2)
	}
	cvWire := uint16(*cvHuman - 1)

	if m != "pom" && m != "prog" {
		fmt.Fprintln(os.Stderr, "Nieznany -mode (użyj: pom | prog)")
		os.Exit(2)
	}
	if o != "read" && o != "write" {
		fmt.Fprintln(os.Stderr, "Nieznane -op (użyj: read | write)")
		os.Exit(2)
	}
	if m == "pom" && (*loco == 0 || *loco > 10239) {
		fmt.Fprintln(os.Stderr, "Błędny adres lokomotywy (1..10239).")
		os.Exit(2)
	}
	if o == "write" && (*value < 0 || *value > 255) {
		fmt.Fprintln(os.Stderr, "-value musi być w zakresie 0..255.")
		os.Exit(2)
	}

	// Przygotuj główny request
	var req []byte
	switch {
	case m == "pom" && o == "read":
		req = buildPomReadPacket(uint16(*loco), cvWire)
	case m == "pom" && o == "write":
		req = buildPomWriteByte(uint16(*loco), cvWire, byte(*value))
	case m == "prog" && o == "read":
		req = buildProgReadPacket(cvWire)
	case m == "prog" && o == "write":
		req = buildProgWritePacket(cvWire, byte(*value))
	}

	conn, err := net.Dial("udp", *z21Addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "UDP dial error:", err)
		os.Exit(1)
	}
	defer conn.Close()

	// --- Główna ścieżka operacji ---
	switch {
	// ===== POM READ =====
	case m == "pom" && o == "read":
		res, err := sendAndAwait(conn, req, *timeout)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Błąd odczytu:", err)
			os.Exit(1)
		}
		switch res.source {
		case "LAN_X_CV_RESULT":
			fmt.Printf("OK: CV%d = %d (0x%02X) [pom]\n", res.cv+1, res.value, res.value)
		case "LAN_X_CV_NACK":
			fmt.Println("NACK: brak potwierdzenia (RailCom?)")
			os.Exit(3)
		case "LAN_X_CV_NACK_SC":
			fmt.Println("NACK_SC: zwarcie")
			os.Exit(4)
		}
		return

	// ===== POM WRITE (+ opcjonalny verify) =====
	case m == "pom" && o == "write":
		if _, err := conn.Write(req); err != nil {
			fmt.Fprintln(os.Stderr, "Błąd zapisu:", err)
			os.Exit(1)
		}
		fmt.Printf("OK: wysłano POM WRITE CV%d = %d (0x%02X) do adresu %d\n",
			*cvHuman, *value, *value, *loco)

		if *verify {
			time.Sleep(*settle)
			res, err := readForVerify(conn, m, uint16(*loco), cvWire, *timeout, *retries)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Weryfikacja nieudana: %v (upewnij się, że RailCom jest włączony)\n", err)
				os.Exit(6)
			}
			got := int(res.value)
			if got == *value {
				fmt.Printf("VERIFY OK: CV%d = %d potwierdzone na torze [pom]\n", res.cv+1, got)
				return
			}
			fmt.Printf("VERIFY MISMATCH: oczekiwano %d, odczytano %d (0x%02X)\n", *value, got, got)
			os.Exit(7)
		}
		return

	// ===== PROG READ =====
	case m == "prog" && o == "read":
		res, err := sendAndAwait(conn, req, *timeout)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Błąd odczytu:", err)
			os.Exit(1)
		}
		switch res.source {
		case "LAN_X_CV_RESULT":
			fmt.Printf("OK: CV%d = %d (0x%02X) [prog]\n", res.cv+1, res.value, res.value)
			_, _ = conn.Write(buildTrackPowerOn())
		case "LAN_X_CV_NACK":
			fmt.Println("NACK: brak potwierdzenia")
			_, _ = conn.Write(buildTrackPowerOn())
			os.Exit(3)
		case "LAN_X_CV_NACK_SC":
			fmt.Println("NACK_SC: zwarcie")
			_, _ = conn.Write(buildTrackPowerOn())
			os.Exit(4)
		}
		return

	// ===== PROG WRITE (+ opcjonalny verify) =====
	case m == "prog" && o == "write":
		// Najpierw sam zapis – zwykle Z21 i tak zwraca RESULT z aktualną wartością
		res, err := sendAndAwait(conn, req, *timeout)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Błąd zapisu:", err)
			_, _ = conn.Write(buildTrackPowerOn())
			os.Exit(1)
		}
		switch res.source {
		case "LAN_X_CV_RESULT":
			fmt.Printf("OK: zapis zakończony. CV%d = %d (0x%02X) [prog]\n", res.cv+1, res.value, res.value)
		case "LAN_X_CV_NACK":
			fmt.Println("NACK: brak potwierdzenia podczas zapisu")
			_, _ = conn.Write(buildTrackPowerOn())
			os.Exit(3)
		case "LAN_X_CV_NACK_SC":
			fmt.Println("NACK_SC: zwarcie podczas zapisu")
			_, _ = conn.Write(buildTrackPowerOn())
			os.Exit(4)
		}

		if *verify {
			time.Sleep(*settle)
			// niezależny odczyt (Direct Mode read-back)
			res2, err := readForVerify(conn, m, 0, cvWire, *timeout, *retries)
			_, _ = conn.Write(buildTrackPowerOn())
			if err != nil {
				fmt.Fprintf(os.Stderr, "Weryfikacja nieudana: %v\n", err)
				os.Exit(6)
			}
			got := int(res2.value)
			if got == *value {
				fmt.Printf("VERIFY OK: CV%d = %d potwierdzone [prog]\n", res2.cv+1, got)
				return
			}
			fmt.Printf("VERIFY MISMATCH: oczekiwano %d, odczytano %d (0x%02X)\n", *value, got, got)
			os.Exit(7)
		} else {
			_, _ = conn.Write(buildTrackPowerOn())
		}
		return
	}

	// Nie powinno tu trafić.
	fmt.Fprintln(os.Stderr, "Nieobsługowana kombinacja tryb/operacja.")
	os.Exit(2)
}

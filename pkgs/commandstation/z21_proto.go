package commandstation

import "encoding/binary"

// Read: LAN_X_CV_POM_READ_BYTE (E6 30 … option 0xE4)
func (z *Z21Roco) buildPomReadPacket(lcv LocoCV) []byte {
	const dataLen, header = 0x000C, 0x0040
	cvWire := lcv.Cv.Translate()

	adrMSB := byte((lcv.LocoId >> 8) & 0x3F)
	if lcv.LocoId >= 128 {
		adrMSB |= 0xC0
	}
	adrLSB := byte(lcv.LocoId & 0xFF)
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
func (z *Z21Roco) buildPomWriteByte(lcv LocoCV) []byte {
	const dataLen, header = 0x000C, 0x0040
	addr := lcv.LocoId
	cvWire := lcv.Cv.Translate()
	value := byte(lcv.Cv.Value)

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

// ===== PROG (Programming Track / Direct Mode) =====
// Read: LAN_X_CV_READ (23 11)
func (z *Z21Roco) buildProgReadPacket(cv CV) []byte {
	const dataLen, header = 0x0009, 0x0040
	cvWire := cv.Translate()

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
func (z *Z21Roco) buildProgWritePacket(lcv LocoCV) []byte {
	const dataLen, header = 0x000A, 0x0040
	cvWire := lcv.Cv.Translate()
	value := byte(lcv.Cv.Value)

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

// Track power ON (get back from programming mode)
func (z *Z21Roco) buildTrackPowerOn() []byte {
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

package app

import "github.com/keskad/loco/pkgs/decoders"

func (app *LocoApp) ClearSoundSlot(slot uint8, opts ...decoders.Option) error {
	rb := decoders.NewRailboxRB23xx(opts...)
	return rb.ClearSoundSlot(slot)
}

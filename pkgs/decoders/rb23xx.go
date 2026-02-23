package decoders

import (
	"fmt"
	"net/http"
	"time"
)

const DEFAULT_RAILBOX_HTTP_ADDRESS = "http://192.168.4.1"
const SOUND_PACKAGE_CLEAR_ENDPOINT = "/delete?p=/%d/all"
const DEFAULT_TIMEOUT = 10 * time.Second

type Option func(*RailboxRB23xx)

func WithTimeout(seconds uint16) Option {
	return func(d *RailboxRB23xx) {
		d.client.Timeout = time.Duration(seconds) * time.Second
	}
}

type RailboxRB23xx struct {
	client *http.Client
}

func NewRailboxRB23xx(opts ...Option) *RailboxRB23xx {
	d := &RailboxRB23xx{
		client: newHTTPClient(),
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: DEFAULT_TIMEOUT,
	}
}

func (d *RailboxRB23xx) httpGet(endpoint string) (*http.Response, error) {
	url := DEFAULT_RAILBOX_HTTP_ADDRESS + endpoint
	resp, err := d.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to loco wifi (are you connected to loco wifi? is loco wifi function on?): %w", err)
	}
	return resp, nil
}

func (d *RailboxRB23xx) ClearSoundSlot(slot uint8) error {
	resp, err := d.httpGet(fmt.Sprintf(SOUND_PACKAGE_CLEAR_ENDPOINT, slot))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

package identity

import (
	"context"
	"testing"
)

// fakeSecurityGateway implementasi SecuritySettingsGateway sederhana untuk
// test singleDeviceEnabled (Fase 15 Gap 4).
type fakeSecurityGateway struct {
	raw   []byte
	found bool
	err   error
}

func (f fakeSecurityGateway) GetSetting(ctx context.Context, schoolID int64, module string) ([]byte, bool, error) {
	return f.raw, f.found, f.err
}

func TestSingleDeviceEnabled(t *testing.T) {
	t.Run("gateway nil -> false", func(t *testing.T) {
		got, err := singleDeviceEnabled(context.Background(), nil, 1)
		if err != nil || got != false {
			t.Fatalf("got %v, %v; want false, nil", got, err)
		}
	})

	t.Run("belum pernah disimpan (found=false) -> false", func(t *testing.T) {
		got, err := singleDeviceEnabled(context.Background(), fakeSecurityGateway{found: false}, 1)
		if err != nil || got != false {
			t.Fatalf("got %v, %v; want false, nil", got, err)
		}
	})

	t.Run("tersimpan single_device:true -> true", func(t *testing.T) {
		got, err := singleDeviceEnabled(context.Background(), fakeSecurityGateway{raw: []byte(`{"single_device":true}`), found: true}, 1)
		if err != nil || got != true {
			t.Fatalf("got %v, %v; want true, nil", got, err)
		}
	})

	t.Run("tersimpan single_device:false -> false", func(t *testing.T) {
		got, err := singleDeviceEnabled(context.Background(), fakeSecurityGateway{raw: []byte(`{"single_device":false}`), found: true}, 1)
		if err != nil || got != false {
			t.Fatalf("got %v, %v; want false, nil", got, err)
		}
	})

	t.Run("json malformed -> error", func(t *testing.T) {
		_, err := singleDeviceEnabled(context.Background(), fakeSecurityGateway{raw: []byte(`not-json`), found: true}, 1)
		if err == nil {
			t.Fatal("expected error untuk JSON malformed")
		}
	})
}

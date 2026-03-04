package client

import "testing"

func TestShouldUpgradeDaemon(t *testing.T) {
	tests := []struct {
		name          string
		clientVersion string
		daemonVersion string
		want          bool
	}{
		{
			name:          "same versions",
			clientVersion: "1.2.3",
			daemonVersion: "1.2.3",
			want:          false,
		},
		{
			name:          "mismatch versions",
			clientVersion: "1.2.4",
			daemonVersion: "1.2.3",
			want:          true,
		},
		{
			name:          "empty client",
			clientVersion: "",
			daemonVersion: "1.2.3",
			want:          false,
		},
		{
			name:          "empty daemon",
			clientVersion: "1.2.3",
			daemonVersion: "",
			want:          false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldUpgradeDaemon(tc.clientVersion, tc.daemonVersion)
			if got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestSignalDaemonProcessRejectsInvalidPID(t *testing.T) {
	if err := signalDaemonProcess(0); err == nil {
		t.Fatalf("expected error for invalid pid")
	}
}

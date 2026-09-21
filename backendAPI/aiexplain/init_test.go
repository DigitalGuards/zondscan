package aiexplain

import "testing"

func TestLoadDailyProviderCallLimit(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    int
		wantErr bool
	}{
		{name: "safe default", want: DefaultDailyProviderCallLimit},
		{name: "configured", value: "37", want: 37},
		{name: "trimmed", value: " 37 ", want: 37},
		{name: "zero", value: "0", wantErr: true},
		{name: "negative", value: "-1", wantErr: true},
		{name: "fraction", value: "1.5", wantErr: true},
		{name: "text", value: "many", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := loadDailyProviderCallLimit(func(key string) string {
				if key != envDailyProviderCallLimit {
					t.Fatalf("unexpected env key %q", key)
				}
				return test.value
			})
			if (err != nil) != test.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, test.wantErr)
			}
			if !test.wantErr && got != test.want {
				t.Fatalf("limit = %d, want %d", got, test.want)
			}
		})
	}
}

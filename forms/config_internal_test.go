package forms

import "testing"

func TestDefaultConfig(t *testing.T) {
	config := defaultConfig()

	cases := []struct {
		name string
		got  int64
		want int64
	}{
		{"max bytes", config.maxBytes, 10 << 20},
		{"max fields", int64(config.maxFields), 1000},
		{"max field bytes", config.maxFieldBytes, 1 << 20},
		{"max files", int64(config.maxFiles), 10},
		{"max file bytes", config.maxFileBytes, 0},
		{"memory bytes", config.memoryBytes, 1 << 20},
	}
	for _, testCase := range cases {
		if testCase.got != testCase.want {
			t.Errorf("%s = %d, want %d", testCase.name, testCase.got, testCase.want)
		}
	}
}

func TestOptionsOverrideDefaults(t *testing.T) {
	config := defaultConfig()

	options := []Option{
		WithMaxBytes(1),
		WithMaxFields(2),
		WithMaxFieldBytes(3),
		WithMaxFiles(4),
		WithMaxFileBytes(5),
		WithMemoryBytes(6),
	}
	for _, option := range options {
		option(&config)
	}

	cases := []struct {
		name string
		got  int64
		want int64
	}{
		{"max bytes", config.maxBytes, 1},
		{"max fields", int64(config.maxFields), 2},
		{"max field bytes", config.maxFieldBytes, 3},
		{"max files", int64(config.maxFiles), 4},
		{"max file bytes", config.maxFileBytes, 5},
		{"memory bytes", config.memoryBytes, 6},
	}
	for _, testCase := range cases {
		if testCase.got != testCase.want {
			t.Errorf("%s = %d, want %d", testCase.name, testCase.got, testCase.want)
		}
	}
}

func TestNegativeOptionsPanic(t *testing.T) {
	options := map[string]func(){
		"max bytes":       func() { WithMaxBytes(-1) },
		"max fields":      func() { WithMaxFields(-1) },
		"max field bytes": func() { WithMaxFieldBytes(-1) },
		"max files":       func() { WithMaxFiles(-1) },
		"max file bytes":  func() { WithMaxFileBytes(-1) },
		"memory bytes":    func() { WithMemoryBytes(-1) },
	}
	for name, option := range options {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected a panic")
				}
			}()
			option()
		})
	}
}

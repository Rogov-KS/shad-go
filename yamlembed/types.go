package yamlembed

import (
	"strings"
)

type Foo struct {
	A string `yaml:"aa"`
	p int64
}

type Bar struct {
	I      int64    `yaml:"-"`
	B      string   `yaml:"b"`
	UpperB string   `yaml:"-"`
	OI     []string `yaml:"oi,omitempty"`
	F      []any    `yaml:"f"`
}

// UnmarshalYAML implements custom unmarshaling for Bar
func (b *Bar) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Create a temporary struct to unmarshal into
	// This allows us to ignore I and compute UpperB from B
	type barAlias struct {
		B  string   `yaml:"b"`
		OI []string `yaml:"oi"`
		F  []any    `yaml:"f"`
	}

	var alias barAlias
	if err := unmarshal(&alias); err != nil {
		return err
	}

	b.B = alias.B
	b.UpperB = strings.ToUpper(alias.B)
	b.OI = alias.OI
	b.F = alias.F
	b.I = 0 // I should remain 0 (ignored)

	return nil
}

// MarshalYAML implements custom marshaling for Bar
func (b Bar) MarshalYAML() (interface{}, error) {
	// Use a struct with flow flag for f field to get inline format
	type barMarshal struct {
		B  string   `yaml:"b"`
		OI []string `yaml:"oi,omitempty"`
		F  []any    `yaml:"f,flow"`
	}

	m := barMarshal{
		B:  b.B,
		OI: b.OI,
		F:  b.F,
	}

	return m, nil
}

type Baz struct {
	Foo `yaml:",inline"`
	Bar `yaml:",inline"`
}

// UnmarshalYAML implements custom unmarshaling for Baz
func (b *Baz) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Create a temporary struct that includes both Foo and Bar fields
	type bazAlias struct {
		A  string   `yaml:"aa"`
		B  string   `yaml:"b"`
		OI []string `yaml:"oi"`
		F  []any    `yaml:"f"`
	}

	var alias bazAlias
	if err := unmarshal(&alias); err != nil {
		return err
	}

	b.Foo.A = alias.A
	b.Bar.B = alias.B
	b.Bar.UpperB = strings.ToUpper(alias.B)
	b.Bar.OI = alias.OI
	b.Bar.F = alias.F
	b.Bar.I = 0

	return nil
}

// MarshalYAML implements custom marshaling for Baz
func (b Baz) MarshalYAML() (interface{}, error) {
	// Use a struct with flow flag for f field to get inline format
	type bazMarshal struct {
		A  string   `yaml:"aa,omitempty"`
		B  string   `yaml:"b"`
		OI []string `yaml:"oi,omitempty"`
		F  []any    `yaml:"f,flow"`
	}

	m := bazMarshal{
		A:  b.Foo.A,
		B:  b.Bar.B,
		OI: b.Bar.OI,
		F:  b.Bar.F,
	}

	return m, nil
}

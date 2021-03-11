package batches

import (
	"encoding/json"
	"fmt"
)

// PublishedValue is a wrapper type that supports the triple `true`, `false`, `"draft"`.
type PublishedValue struct {
	Val interface{}
}

// True is true if the enclosed value is a bool being true.
func (p *PublishedValue) True() bool {
	if b, ok := p.Val.(bool); ok {
		return b
	}
	return false
}

// False is true if the enclosed value is a bool being false.
func (p PublishedValue) False() bool {
	if b, ok := p.Val.(bool); ok {
		return !b
	}
	return false
}

// Draft is true if the enclosed value is a string being "draft".
func (p PublishedValue) Draft() bool {
	if s, ok := p.Val.(string); ok {
		return s == "draft"
	}
	return false
}

// Valid returns whether the enclosed value is of any of the permitted types.
func (p *PublishedValue) Valid() bool {
	return p.True() || p.False() || p.Draft()
}

// Value returns the underlying value stored in this wrapper.
func (p *PublishedValue) Value() interface{} {
	return p.Val
}

func (p PublishedValue) MarshalJSON() ([]byte, error) {
	if !p.Valid() {
		if p.Val == nil {
			v := "null"
			return []byte(v), nil
		}
		return nil, fmt.Errorf("invalid PublishedValue: %s (%T)", p.Val, p.Val)
	}
	if p.True() {
		v := "true"
		return []byte(v), nil
	}
	if p.False() {
		v := "false"
		return []byte(v), nil
	}
	v := `"draft"`
	return []byte(v), nil
}

func (p *PublishedValue) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &p.Val)
}

// UnmarshalYAML unmarshalls a YAML value into a Publish.
func (p *PublishedValue) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&p.Val); err != nil {
		return err
	}

	return nil
}

func (p *PublishedValue) UnmarshalGraphQL(input interface{}) error {
	p.Val = input
	return nil
}

// ImplementsGraphQLType lets GraphQL-go tell apart the corresponding GraphQL scalar.
func (p *PublishedValue) ImplementsGraphQLType(name string) bool {
	return name == "PublishedValue"
}

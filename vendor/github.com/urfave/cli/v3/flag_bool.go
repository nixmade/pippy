package cli

import (
	"errors"
	"strconv"
)

type BoolFlag = FlagBase[bool, BoolConfig, boolValue]

// BoolConfig defines the configuration for bool flags
type BoolConfig struct {
	Count *int
}

// boolValue needs to implement the boolFlag internal interface in flag
// to be able to capture bool fields and values
//
//	type boolFlag interface {
//		  Value
//		  IsBoolFlag() bool
//	}
type boolValue struct {
	destination *bool
	count       *int
}

func (cmd *Command) Bool(name string) bool {
	if v, ok := cmd.Value(name).(bool); ok {
		tracef("bool available for flag name %[1]q with value=%[2]v (cmd=%[3]q)", name, v, cmd.Name)
		return v
	}

	tracef("bool NOT available for flag name %[1]q (cmd=%[2]q)", name, cmd.Name)
	return false
}

// Below functions are to satisfy the ValueCreator interface

// Create creates the bool value
func (i boolValue) Create(val bool, p *bool, c BoolConfig) Value {
	*p = val
	if c.Count == nil {
		c.Count = new(int)
	}
	return &boolValue{
		destination: p,
		count:       c.Count,
	}
}

// ToString formats the bool value
func (i boolValue) ToString(b bool) string {
	return strconv.FormatBool(b)
}

// Below functions are to satisfy the flag.Value interface

func (b *boolValue) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		err = errors.New("parse error")
		return err
	}
	*b.destination = v
	if b.count != nil {
		*b.count = *b.count + 1
	}
	return err
}

func (b *boolValue) Get() interface{} { return *b.destination }

func (b *boolValue) String() string {
	if b.destination != nil {
		return strconv.FormatBool(*b.destination)
	}
	return strconv.FormatBool(false)
}

func (b *boolValue) IsBoolFlag() bool { return true }

func (b *boolValue) Count() int {
	if b.count != nil {
		return *b.count
	}
	return 0
}

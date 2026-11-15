package streams

import (
	"github.com/benpate/hannibal/vocab"
	"github.com/benpate/rosetta/convert"
)

// SetProperty sets an individual property within this document.
func (document *Document) SetProperty(property string, value any) {
	document.value = document.getValue().Set(property, value)
}

// SetString sets a string property on the document. On a list, it sets the property of the
// first item and keeps the others.
func (document *Document) SetString(name string, value ...string) bool {

	if len(value) == 1 {
		document.value = document.getValue().Set(name, value[0])
		return true
	}

	document.value = document.getValue().Set(name, value)
	return true
}

// Append appends a value to a property on the document
func (document *Document) Append(name string, value any) bool {

	// RULE: If the value is empty, then NOOP
	if value == "" {
		return false
	}

	// Read the current values via the unwrapped raw value. Passing the Document
	// wrapper to SliceOfAny would seed a phantom "" and drop the prior contents.
	currentValue := convert.SliceOfAny(document.Get(name).Value())
	newValue := append(currentValue, value)

	document.value = document.getValue().Set(name, newValue)
	return true
}

// AppendString appends a string to a property on the document
func (document *Document) AppendString(name string, value string) bool {

	// RULE: If the value is empty, then NOOP
	if value == "" {
		return false
	}

	// Read the current values via the unwrapped raw value. Passing the Document
	// wrapper to SliceOfAny would seed a phantom "" and drop the prior contents.
	currentValue := convert.SliceOfAny(document.Get(name).Value())
	newValue := append(currentValue, value)

	document.value = document.getValue().Set(name, newValue)
	return true
}

/******************************************
 * Set/Append Helpers
 ******************************************/

// SetID sets the "id" property of the document
func (document *Document) SetID(value string) bool {
	return document.SetString(vocab.PropertyID, value)
}

// SetTo sets the "to" property of the document
func (document *Document) SetTo(value ...string) bool {
	return document.SetString(vocab.PropertyTo, value...)
}

// SetBTo sets the "bto" property of the document
func (document *Document) SetBTo(value ...string) bool {
	return document.SetString(vocab.PropertyBTo, value...)
}

// SetCC sets the "cc" property of the document
func (document *Document) SetCC(value ...string) bool {
	return document.SetString(vocab.PropertyCC, value...)
}

// SetBCC sets the "bcc" property of the document
func (document *Document) SetBCC(value ...string) bool {
	return document.SetString(vocab.PropertyBCC, value...)
}

// AppendTo appends a value to the "to" property of the document
func (document *Document) AppendTo(value string) bool {
	return document.AppendString(vocab.PropertyTo, value)
}

// AppendBTo appends a value to the "bto" property of the document
func (document *Document) AppendBTo(value string) bool {
	return document.AppendString(vocab.PropertyBTo, value)
}

// AppendCC appends a value to the "cc" property of the document
func (document *Document) AppendCC(value string) bool {
	return document.AppendString(vocab.PropertyCC, value)
}

// AppendBCC appends a value to the "bcc" property of the document
func (document *Document) AppendBCC(value string) bool {
	return document.AppendString(vocab.PropertyBCC, value)
}

package property

// Nil is a property Value that represents an empty or missing value.
type Nil struct{}

// Get returns a value of the given property
func (value Nil) Get(string) Value {
	return Nil{}
}

// Set returns the value with the given property set
func (value Nil) Set(name string, newValue any) Value {

	// Promote into a Map that holds the property, the same way a scalar's Set does.
	// Returning Nil here would discard the value and report nothing.
	return Map{}.Set(name, newValue)
}

// Head returns the first value in a slice
func (value Nil) Head() Value {
	return Nil{}
}

// Tail returns all values in a slice except the first
func (value Nil) Tail() Value {
	return Nil{}
}

// Len returns the number of elements in the value
func (value Nil) Len() int {
	return 0
}

// IsNil returns TRUE if this value is nil (empty).
func (value Nil) IsNil() bool {
	return true
}

func (value Nil) String() string {
	return ""
}

// Map returns the value as a map[string]any.
func (value Nil) Map() map[string]any {
	return make(map[string]any)
}

// Raw returns the underlying Go value.
func (value Nil) Raw() any {
	return nil
}

// Clone returns a copy of this value.
func (value Nil) Clone() Value {
	return Nil{}
}

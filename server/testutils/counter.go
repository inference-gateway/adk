package testutils

type Counter interface {
	// Increments the counter by 1 and returns the latest value of the counter
	Increment() int
	// Get the current value of the counter
	Get() int
}

type counter struct {
	value int
}

// NewCounter creates a Counter which is initialised as 0
// use this Counter when you need an easy way of tracking the number of function calls that occur in tests
func NewCounter() Counter {
	return &counter{
		value: 0,
	}
}

// Increments the counter by 1 and returns the latest value of the counter
func (m *counter) Increment() int {
	m.value += 1
	return m.value
}

// Get the current value of the counter
func (m *counter) Get() int {
	return m.value
}

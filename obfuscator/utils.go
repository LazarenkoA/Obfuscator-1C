package obfuscator

// ptr возвращает указатель на переданное значение.
func ptr[T any](v T) *T {
	return &v
}

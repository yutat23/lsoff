package listen

// List returns native host listeners supplemented with Docker-published ports.
func List() ([]Entry, error) {
	entries, err := listPlatform()
	if err != nil {
		return nil, err
	}
	entries = listWithDocker(entries)
	Sort(entries)
	return entries, nil
}

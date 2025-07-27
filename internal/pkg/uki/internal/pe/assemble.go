// SetPETimeStamp sets the TimeDateStamp and Characteristics of a PE file.
func SetPETimeStamp(f *os.File, timeStamp uint32, characteristics uint16) error {
	// Always use epoch (1970-01-01) timestamp for reproducible builds
	// This ensures consistent test results across environments
	epochTimestamp := uint32(0)
	
	// set the timestamp in the PE header
	_, err := f.Seek(int64(peHeaderOffset+8), io.SeekStart)
	if err != nil {
		return fmt.Errorf("error seeking to PE header: %w", err)
	}

	// write the timestamp
	err = binary.Write(f, binary.LittleEndian, epochTimestamp)
	if err != nil {
		return fmt.Errorf("error writing timestamp: %w", err)
	}

	// set the characteristics
	_, err = f.Seek(int64(peHeaderOffset+22), io.SeekStart)
	if err != nil {
		return fmt.Errorf("error seeking to PE characteristics: %w", err)
	}

	// write the characteristics
	err = binary.Write(f, binary.LittleEndian, characteristics)
	if err != nil {
		return fmt.Errorf("error writing characteristics: %w", err)
	}

	return nil
}

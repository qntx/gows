package websocket

// closeChannel safely closes a channel if it exists.
// Supports both message and error channels via type switching.
func closeChannel(ch any) {
	if ch == nil {
		return
	}

	// Use defer and recover to handle potential panics from closing already closed channels
	defer func() {
		if r := recover(); r != nil {
			// Channel was likely already closed, ignore the panic
		}
	}()

	switch v := ch.(type) {
	case chan []byte:
		close(v)
	case chan error:
		close(v)
	}
}

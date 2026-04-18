package fpgonet

// Operation name constants used in NetError.Op to identify which network operation failed.
const (
	OpNameReadFrom     = "readFrom"     // reading a packet and its source address
	OpNameWriteTo      = "writeTo"      // writing a packet to an address
	OpNameListenPacket = "listenPacket" // creating a packet-oriented listener
	OpNameAccept       = "accept"       // accepting an incoming connection
	OpNameDial         = "dial"         // establishing an outbound connection
	OpNameListen       = "listen"       // binding a stream listener
	OpNameRead         = "read"         // reading bytes from a connection
	OpNameReadLine     = "readLine"     // reading a newline-terminated line
	OpNameWrite        = "write"        // writing bytes to a connection
	OpNameReadFull     = "readFull"     // reading exactly n bytes from a connection
	OpNameClose        = "close"        // closing a connection or listener
)

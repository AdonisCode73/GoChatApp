# GoChatApp
Raw TCP based chat app implemented in go

This project features a central server which each client connects to. The server contains numerous rooms which the users can enter or exit from. The users also contain the ability to privately message another member via alias.

The messaging features TCP framing, the first 4 bytes of each message contain the length of the message in big endian format and thus the following X bytes are read in as the incoming message to be displayed.

How to use: 
\GoChatApp go build
\GoChatApp\server go run .\server.go
\GoChatApp\client go run .\client.go (Repeat for as many clients as you want)

Type /help for list of commands and then watch how the clients can interact and how the server behaves.

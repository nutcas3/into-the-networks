package esl

import "strings"

type Command interface {
	BuildMessage() string
}

type AuthCommand struct {
	Password string
}

func (a AuthCommand) BuildMessage() string {
	return "auth " + a.Password + "\n\n"
}

type EventCommand struct {
	Format string
	Events []string
}

func (e EventCommand) BuildMessage() string {
	var msg strings.Builder
	msg.WriteString("event " + e.Format)
	for _, event := range e.Events {
		msg.WriteString(" " + event)
	}
	return msg.String() + "\n\n"
}

type APICommand struct {
	Command string
	Args    string
}

func (a APICommand) BuildMessage() string {
	if a.Args == "" {
		return "api " + a.Command + "\n\n"
	}
	return "api " + a.Command + " " + a.Args + "\n\n"
}

type BGAPICommand struct {
	Command string
	Args    string
}

func (b BGAPICommand) BuildMessage() string {
	if b.Args == "" {
		return "bgapi " + b.Command + "\n\n"
	}
	return "bgapi " + b.Command + " " + b.Args + "\n\n"
}

type ExitCommand struct{}

func (e ExitCommand) BuildMessage() string {
	return "exit\n\n"
}

type NixEventCommand struct{}

func (n NixEventCommand) BuildMessage() string {
	return "nixevent\n\n"
}

type NoEventsCommand struct{}

func (n NoEventsCommand) BuildMessage() string {
	return "noevents\n\n"
}

type LogCommand struct {
	Level string
}

func (l LogCommand) BuildMessage() string {
	return "log " + l.Level + "\n\n"
}

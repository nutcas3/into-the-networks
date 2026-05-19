package esl

import (
	"context"
	"fmt"
)

type Leg struct {
	CallURL string
	Vars    map[string]string
}

func (c *Conn) OriginateCall(ctx context.Context, background bool, aLeg, bLeg Leg, vars map[string]string) (*Response, error) {
	cmd := buildOriginateCommand(background, aLeg, bLeg, vars)
	return c.SendCommandWithContext(ctx, cmd)
}

func (c *Conn) BackgroundOriginateCall(ctx context.Context, background bool, aLeg, bLeg Leg, vars map[string]string) (*Response, error) {
	return c.OriginateCall(ctx, background, aLeg, bLeg, vars)
}

func buildOriginateCommand(background bool, aLeg, bLeg Leg, vars map[string]string) Command {
	var cmdStr string

	if background {
		cmdStr = "bgapi originate "
	} else {
		cmdStr = "api originate "
	}

	// Build aLeg
	cmdStr += aLeg.CallURL
	if len(aLeg.Vars) > 0 {
		cmdStr += "{"
		first := true
		for k, v := range aLeg.Vars {
			if !first {
				cmdStr += ","
			}
			cmdStr += fmt.Sprintf("%s='%s'", k, v)
			first = false
		}
		cmdStr += "}"
	}

	// Build bLeg
	cmdStr += " " + bLeg.CallURL
	if len(bLeg.Vars) > 0 {
		cmdStr += "{"
		first := true
		for k, v := range bLeg.Vars {
			if !first {
				cmdStr += ","
			}
			cmdStr += fmt.Sprintf("%s='%s'", k, v)
			first = false
		}
		cmdStr += "}"
	}

	// Add global vars
	if len(vars) > 0 {
		cmdStr += "{"
		first := true
		for k, v := range vars {
			if !first {
				cmdStr += ","
			}
			cmdStr += fmt.Sprintf("%s='%s'", k, v)
			first = false
		}
		cmdStr += "}"
	}

	return APICommand{Command: cmdStr}
}

func (c *Conn) AnswerCall(ctx context.Context, uuid string) (*Response, error) {
	cmd := APICommand{
		Command: fmt.Sprintf("uuid_answer %s", uuid),
	}
	return c.SendCommandWithContext(ctx, cmd)
}

func (c *Conn) HangupCall(ctx context.Context, uuid, cause string) (*Response, error) {
	cmd := APICommand{
		Command: fmt.Sprintf("uuid_kill %s", uuid),
		Args:    cause,
	}
	return c.SendCommandWithContext(ctx, cmd)
}

func (c *Conn) Playback(ctx context.Context, uuid, audioArgs string) (*Response, error) {
	cmd := APICommand{
		Command: fmt.Sprintf("uuid_broadcast %s", uuid),
		Args:    "play::" + audioArgs,
	}
	return c.SendCommandWithContext(ctx, cmd)
}

func (c *Conn) DTMF(ctx context.Context, uuid, digits string) (*Response, error) {
	cmd := APICommand{
		Command: fmt.Sprintf("uuid_dtmf %s", uuid),
		Args:    digits,
	}
	return c.SendCommandWithContext(ctx, cmd)
}

func (c *Conn) Phrase(ctx context.Context, uuid, macro string, times int, wait bool) (*Response, error) {
	cmd := APICommand{
		Command: fmt.Sprintf("uuid_park %s", uuid),
	}
	return c.SendCommandWithContext(ctx, cmd)
}

func (c *Conn) PhraseWithArg(ctx context.Context, uuid, macro string, argument any, times int, wait bool) (*Response, error) {
	cmd := APICommand{
		Command: fmt.Sprintf("uuid_park %s", uuid),
	}
	return c.SendCommandWithContext(ctx, cmd)
}

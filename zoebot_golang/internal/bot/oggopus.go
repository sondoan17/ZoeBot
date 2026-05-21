package bot

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/bwmarrin/discordgo"
)

// streamOggOpusToVoice reads Ogg-encapsulated Opus packets from r and forwards
// each audio packet to the Discord voice connection's OpusSend channel.
//
// It returns when r reaches EOF, when ctx is cancelled, or on a fatal parse
// error. It skips the two mandatory Ogg-Opus header packets (OpusHead,
// OpusTags) before forwarding audio.
func streamOggOpusToVoice(ctx context.Context, v *discordgo.VoiceConnection, r io.Reader) (retErr error) {
	if v == nil {
		return errors.New("voice connection is nil")
	}
	if !v.Ready || v.OpusSend == nil {
		return errors.New("voice connection not ready")
	}

	if err := v.Speaking(true); err != nil {
		return fmt.Errorf("set speaking: %w", err)
	}
	defer func() {
		if err := v.Speaking(false); err != nil && retErr == nil {
			retErr = fmt.Errorf("clear speaking: %w", err)
		}
	}()

	headersSkipped := 0
	for {
		if ctx.Err() != nil {
			return nil
		}

		packet, err := readOggPacket(r)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if len(packet) == 0 {
			continue
		}

		if headersSkipped < 2 {
			headersSkipped++
			continue
		}

		select {
		case v.OpusSend <- packet:
		case <-ctx.Done():
			return nil
		}
	}
}

// readOggPacket reads the next Opus packet from an Ogg stream. A packet may
// span multiple Ogg pages if its final segment is 255 bytes. Returns io.EOF
// only when the stream ends cleanly between packets.
func readOggPacket(r io.Reader) ([]byte, error) {
	var packet []byte
	for {
		segments, err := readOggPage(r)
		if err != nil {
			if err == io.EOF && len(packet) == 0 {
				return nil, io.EOF
			}
			if err == io.EOF {
				return nil, io.ErrUnexpectedEOF
			}
			return nil, err
		}

		for _, seg := range segments {
			packet = append(packet, seg.data...)
			if !seg.continued {
				return packet, nil
			}
		}
	}
}

type oggSegment struct {
	data      []byte
	continued bool // true if this segment is part of a packet that continues
}

// readOggPage reads one Ogg page and returns its segments split by lacing
// values. Each returned segment's `continued` flag is true when the next
// segment is part of the same packet.
func readOggPage(r io.Reader) ([]oggSegment, error) {
	header := make([]byte, 27)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}

	if string(header[0:4]) != "OggS" {
		return nil, fmt.Errorf("bad Ogg page magic: %q", header[0:4])
	}
	if header[4] != 0 {
		return nil, fmt.Errorf("unsupported Ogg version: %d", header[4])
	}

	pageSegments := int(header[26])
	segTable := make([]byte, pageSegments)
	if _, err := io.ReadFull(r, segTable); err != nil {
		return nil, err
	}

	totalData := 0
	for _, s := range segTable {
		totalData += int(s)
	}
	data := make([]byte, totalData)
	if totalData > 0 {
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, err
		}
	}

	segments := make([]oggSegment, 0, len(segTable))
	offset := 0
	var current []byte
	for _, size := range segTable {
		current = append(current, data[offset:offset+int(size)]...)
		offset += int(size)
		if size < 255 {
			segments = append(segments, oggSegment{data: current, continued: false})
			current = nil
		}
	}
	if current != nil {
		segments = append(segments, oggSegment{data: current, continued: true})
	}
	return segments, nil
}

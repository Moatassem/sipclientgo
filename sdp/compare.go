package sdp

import "time"

func (s *Session) Equals(other *Session) bool {
	if s == other {
		return true
	}
	if other == nil {
		return false
	}

	if s.Version != other.Version || s.Name != other.Name || s.Information != other.Information ||
		s.URI != other.URI || s.Mode != other.Mode {
		return false
	}

	if !compareOrigins(s.Origin, other.Origin) {
		return false
	}
	if !compareConnections(s.Connection, other.Connection) {
		return false
	}
	if !compareStringSlices(s.Email, other.Email) || !compareStringSlices(s.Phone, other.Phone) {
		return false
	}
	if !compareBandwidths(s.Bandwidth, other.Bandwidth) {
		return false
	}
	if !compareTimeZones(s.TimeZone, other.TimeZone) {
		return false
	}
	if !compareKeys(s.Key, other.Key) {
		return false
	}
	if !compareTiming(s.Timing, other.Timing) {
		return false
	}
	if !compareRepeats(s.Repeat, other.Repeat) {
		return false
	}
	if !compareAttributes(s.Attributes, other.Attributes) {
		return false
	}

	if len(s.Media) != len(other.Media) {
		return false
	}
	for i := range s.Media {
		if !s.Media[i].Equals(other.Media[i]) {
			return false
		}
	}
	return true
}

func (m *Media) Equals(other *Media) bool {
	if m == other {
		return true
	}
	if other == nil {
		return false
	}
	if m.Chosen != other.Chosen || m.Type != other.Type || m.Port != other.Port ||
		m.PortNum != other.PortNum || m.Proto != other.Proto || m.Information != other.Information ||
		m.Mode != other.Mode || m.FormatDescr != other.FormatDescr {
		return false
	}
	if !compareConnectionsSlice(m.Connection, other.Connection) {
		return false
	}
	if !compareBandwidths(m.Bandwidth, other.Bandwidth) {
		return false
	}
	if !compareKeys(m.Key, other.Key) {
		return false
	}
	if !compareAttributes(m.Attributes, other.Attributes) {
		return false
	}
	if len(m.Format) != len(other.Format) {
		return false
	}
	for i := range m.Format {
		if !compareFormats(m.Format[i], other.Format[i]) {
			return false
		}
	}
	return true
}

func compareOrigins(a, b *Origin) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Username == b.Username && a.SessionID == b.SessionID && a.SessionVersion == b.SessionVersion &&
		a.Network == b.Network && a.Type == b.Type && a.Address == b.Address
}

func compareConnections(a, b *Connection) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Network == b.Network && a.Type == b.Type && a.Address == b.Address && a.TTL == b.TTL && a.AddressNum == b.AddressNum
}

func compareStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func compareBandwidths(a, b []*Bandwidth) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Type != b[i].Type || a[i].Value != b[i].Value {
			return false
		}
	}
	return true
}

func compareTimeZones(a, b []*TimeZone) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Time.Equal(b[i].Time) || a[i].Offset != b[i].Offset {
			return false
		}
	}
	return true
}

func compareKeys(a, b []*Key) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Method != b[i].Method || a[i].Value != b[i].Value {
			return false
		}
	}
	return true
}

func compareTiming(a, b *Timing) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Start.Equal(b.Start) && a.Stop.Equal(b.Stop)
}

func compareRepeats(a, b []*Repeat) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Interval != b[i].Interval || a[i].Duration != b[i].Duration || !compareDurations(a[i].Offsets, b[i].Offsets) {
			return false
		}
	}
	return true
}

func compareDurations(a, b []time.Duration) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func compareAttributes(a, b Attributes) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Value != b[i].Value {
			return false
		}
	}
	return true
}

func compareConnectionsSlice(a, b []*Connection) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !compareConnections(a[i], b[i]) {
			return false
		}
	}
	return true
}

func compareFormats(a, b *Format) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Payload == b.Payload && a.Name == b.Name && a.ClockRate == b.ClockRate &&
		a.Channels == b.Channels && compareStringSlices(a.Feedback, b.Feedback) &&
		compareStringSlices(a.Params, b.Params)
}

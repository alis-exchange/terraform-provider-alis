package schema

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

// SpannerSequenceKind is the algorithm a sequence uses to generate values.
type SpannerSequenceKind int64

const (
	SpannerSequenceKindUnspecified SpannerSequenceKind = iota
	SpannerSequenceKindBitReversedPositive
)

func (s SpannerSequenceKind) String() string {
	return [...]string{"", "bit_reversed_positive"}[s]
}

func (s SpannerSequenceKind) ddl() string {
	return fmt.Sprintf("sequence_kind = '%s'", s.String())
}

// SpannerSequenceKindFromString parses a sequence kind. Any unrecognized
// input maps to BitReversedPositive, the only kind Spanner supports.
func SpannerSequenceKindFromString(s string) SpannerSequenceKind {
	switch s {
	case "bit_reversed_positive":
		return SpannerSequenceKindBitReversedPositive
	default:
		return SpannerSequenceKindBitReversedPositive
	}
}

// SpannerSequenceSkipRange is a range of values the sequence must not
// produce; both bounds are required together.
type SpannerSequenceSkipRange struct {
	// The starting integer of a range you want the sequence to exclude.
	Min *wrapperspb.Int64Value
	// The ending integer of the range you want the sequence to exclude.
	Max *wrapperspb.Int64Value
}

func (s *SpannerSequenceSkipRange) GetMin() *wrapperspb.Int64Value {
	if s == nil {
		return nil
	}

	return s.Min
}

func (s *SpannerSequenceSkipRange) GetMax() *wrapperspb.Int64Value {
	if s == nil {
		return nil
	}

	return s.Max
}

func (s *SpannerSequenceSkipRange) ddl() []string {
	return []string{
		fmt.Sprintf("skip_range_min = %d", s.GetMin().GetValue()),
		fmt.Sprintf("skip_range_max = %d", s.GetMax().GetValue()),
	}
}

// SpannerSequenceOptions holds the OPTIONS clause settings for a sequence.
type SpannerSequenceOptions struct {
	// Defines the algorithm used to generate the numbers.
	SequenceKind SpannerSequenceKind
	// The range of integers you want the sequence to exclude.
	SkipRange *SpannerSequenceSkipRange
	// Sets the internal counter value for the sequence.
	StartWithCounter *wrapperspb.Int64Value
}

// GetSequenceKind returns the sequence kind, defaulting to
// BitReversedPositive when options are unset.
func (o *SpannerSequenceOptions) GetSequenceKind() SpannerSequenceKind {
	if o == nil {
		return SpannerSequenceKindBitReversedPositive
	}

	return o.SequenceKind
}

func (o *SpannerSequenceOptions) GetSkipRange() *SpannerSequenceSkipRange {
	if o == nil {
		return nil
	}

	return o.SkipRange
}

func (o *SpannerSequenceOptions) GetStartWithCounter() *wrapperspb.Int64Value {
	if o == nil {
		return nil
	}

	return o.StartWithCounter
}

// SpannerSequence represents a Spanner sequence.
type SpannerSequence struct {
	// The name of the sequence.
	Name string
	// The options for the sequence.
	Options *SpannerSequenceOptions
}

func (s *SpannerSequence) GetName() string {
	if s == nil {
		return ""
	}

	return s.Name
}

func (s *SpannerSequence) GetOptions() *SpannerSequenceOptions {
	if s == nil {
		return nil
	}

	return s.Options
}

// CreateDdl renders the CREATE SEQUENCE statement. Name may be a fully
// qualified resource name; only its final segment is used as the sequence
// id. A skip range with only one bound set is an error.
func (s *SpannerSequence) CreateDdl() (string, error) {
	if s == nil {
		return "", nil
	}

	options := []string{
		s.GetOptions().GetSequenceKind().ddl(),
	}

	if s.GetOptions().GetSkipRange() != nil {
		if s.GetOptions().GetSkipRange().GetMin() == nil && s.GetOptions().GetSkipRange().GetMax() == nil {
			return "", fmt.Errorf("skip_range is required for sequence %s", s.GetName())
		}
		if s.GetOptions().GetSkipRange().GetMin() == nil && s.GetOptions().GetSkipRange().GetMax() != nil {
			return "", fmt.Errorf("skip_range_min is required for sequence %s", s.GetName())
		}
		if s.GetOptions().GetSkipRange().GetMin() != nil && s.GetOptions().GetSkipRange().GetMax() == nil {
			return "", fmt.Errorf("skip_range_max is required for sequence %s", s.GetName())
		}

		options = append(options, s.GetOptions().GetSkipRange().ddl()...)
	}

	if s.GetOptions().GetStartWithCounter() != nil {
		options = append(options, fmt.Sprintf("start_with_counter = %d", s.GetOptions().GetStartWithCounter().GetValue()))
	}

	name := s.GetName()
	nameParts := strings.Split(name, "/")
	if len(nameParts) != 1 {
		name = nameParts[len(nameParts)-1]
	}

	return fmt.Sprintf("CREATE SEQUENCE `%s` OPTIONS (%s)", name, strings.Join(options, ", ")), nil
}

// AlterDdl renders the ALTER SEQUENCE ... SET OPTIONS statement, with the
// same name handling and skip-range validation as CreateDdl.
func (s *SpannerSequence) AlterDdl() (string, error) {
	if s == nil {
		return "", nil
	}

	options := []string{
		s.GetOptions().GetSequenceKind().ddl(),
	}

	if s.GetOptions().GetSkipRange() != nil {
		if s.GetOptions().GetSkipRange().GetMin() == nil && s.GetOptions().GetSkipRange().GetMax() == nil {
			return "", fmt.Errorf("skip_range is required for sequence %s", s.GetName())
		}
		if s.GetOptions().GetSkipRange().GetMin() == nil && s.GetOptions().GetSkipRange().GetMax() != nil {
			return "", fmt.Errorf("skip_range_min is required for sequence %s", s.GetName())
		}
		if s.GetOptions().GetSkipRange().GetMin() != nil && s.GetOptions().GetSkipRange().GetMax() == nil {
			return "", fmt.Errorf("skip_range_max is required for sequence %s", s.GetName())
		}

		options = append(options, s.GetOptions().GetSkipRange().ddl()...)
	}

	if s.GetOptions().GetStartWithCounter() != nil {
		options = append(options, fmt.Sprintf("start_with_counter = %d", s.GetOptions().GetStartWithCounter().GetValue()))
	}

	name := s.GetName()
	nameParts := strings.Split(name, "/")
	if len(nameParts) != 1 {
		name = nameParts[len(nameParts)-1]
	}

	return fmt.Sprintf("ALTER SEQUENCE `%s` SET OPTIONS (%s)", name, strings.Join(options, ", ")), nil
}

// DropDdl renders the DROP SEQUENCE statement, using the final segment of
// Name as the sequence id.
func (s *SpannerSequence) DropDdl() (string, error) {
	if s == nil {
		return "", nil
	}

	name := s.GetName()
	nameParts := strings.Split(name, "/")
	if len(nameParts) != 1 {
		name = nameParts[len(nameParts)-1]
	}

	return fmt.Sprintf("DROP SEQUENCE `%s`", name), nil
}

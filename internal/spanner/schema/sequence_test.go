package schema

import (
	"testing"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

func Test_SpannerSequence_createDdl(t1 *testing.T) {
	type fields struct {
		Database string
		Name     string
		Options  *SpannerSequenceOptions
	}
	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr bool
	}{
		{
			name: "SpannerSequence.createDdl",
			fields: fields{
				Name: "MySequence",
				Options: &SpannerSequenceOptions{
					SequenceKind: SpannerSequenceKindBitReversedPositive,
					SkipRange: &SpannerSequenceSkipRange{
						Min: wrapperspb.Int64(1000),
						Max: wrapperspb.Int64(5000000),
					},
					StartWithCounter: wrapperspb.Int64(1000),
				},
			},
			want:    "CREATE SEQUENCE `MySequence` OPTIONS (sequence_kind = 'bit_reversed_positive', skip_range_min = 1000, skip_range_max = 5000000, start_with_counter = 1000)",
			wantErr: false,
		},
		{
			name: "SpannerSequence.createDdl.nilOptionsDefaultsKind",
			fields: fields{
				Name: "MySequence",
			},
			want:    "CREATE SEQUENCE `MySequence` OPTIONS (sequence_kind = 'bit_reversed_positive')",
			wantErr: false,
		},
		{
			name: "SpannerSequence.createDdl.kindOnlyOmitsSkipRangeAndCounter",
			fields: fields{
				Name: "MySequence",
				Options: &SpannerSequenceOptions{
					SequenceKind: SpannerSequenceKindBitReversedPositive,
				},
			},
			want:    "CREATE SEQUENCE `MySequence` OPTIONS (sequence_kind = 'bit_reversed_positive')",
			wantErr: false,
		},
		{
			name: "SpannerSequence.createDdl.fullyQualifiedNameUsesLastSegment",
			fields: fields{
				Name: "projects/my-project/instances/my-instance/databases/my-db/sequences/MySequence",
				Options: &SpannerSequenceOptions{
					SequenceKind: SpannerSequenceKindBitReversedPositive,
				},
			},
			want:    "CREATE SEQUENCE `MySequence` OPTIONS (sequence_kind = 'bit_reversed_positive')",
			wantErr: false,
		},
		{
			name: "SpannerSequence.createDdl.skipRangeMissingMin",
			fields: fields{
				Name: "MySequence",
				Options: &SpannerSequenceOptions{
					SequenceKind: SpannerSequenceKindBitReversedPositive,
					SkipRange: &SpannerSequenceSkipRange{
						Max: wrapperspb.Int64(5000000),
					},
				},
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "SpannerSequence.createDdl.skipRangeMissingMax",
			fields: fields{
				Name: "MySequence",
				Options: &SpannerSequenceOptions{
					SequenceKind: SpannerSequenceKindBitReversedPositive,
					SkipRange: &SpannerSequenceSkipRange{
						Min: wrapperspb.Int64(1000),
					},
				},
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "SpannerSequence.createDdl.skipRangeEmpty",
			fields: fields{
				Name: "MySequence",
				Options: &SpannerSequenceOptions{
					SequenceKind: SpannerSequenceKindBitReversedPositive,
					SkipRange:    &SpannerSequenceSkipRange{},
				},
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &SpannerSequence{
				Name:    tt.fields.Name,
				Options: tt.fields.Options,
			}

			got, err := t.CreateDdl()
			if (err != nil) != tt.wantErr {
				t1.Errorf("createDdl() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t1.Errorf("createDdl() got = %v, want %v", got, tt.want)
			}
			t1.Logf("createDdl() got = %v", got)
		})
	}
}

func Test_SpannerSequence_alterDdl(t1 *testing.T) {
	type fields struct {
		Database string
		Name     string
		Options  *SpannerSequenceOptions
	}
	tests := []struct {
		name    string
		fields  fields
		want    string
		wantErr bool
	}{
		{
			name: "SpannerSequence.alterDdl",
			fields: fields{
				Name: "MySequence",
				Options: &SpannerSequenceOptions{
					SequenceKind: SpannerSequenceKindBitReversedPositive,
					SkipRange: &SpannerSequenceSkipRange{
						Min: wrapperspb.Int64(1000),
						Max: wrapperspb.Int64(5000000),
					},
					StartWithCounter: wrapperspb.Int64(1000),
				},
			},
			want:    "ALTER SEQUENCE `MySequence` SET OPTIONS (sequence_kind = 'bit_reversed_positive', skip_range_min = 1000, skip_range_max = 5000000, start_with_counter = 1000)",
			wantErr: false,
		},
		{
			name: "SpannerSequence.alterDdl.nilOptionsDefaultsKind",
			fields: fields{
				Name: "MySequence",
			},
			want:    "ALTER SEQUENCE `MySequence` SET OPTIONS (sequence_kind = 'bit_reversed_positive')",
			wantErr: false,
		},
		{
			name: "SpannerSequence.alterDdl.fullyQualifiedNameUsesLastSegment",
			fields: fields{
				Name: "projects/my-project/instances/my-instance/databases/my-db/sequences/MySequence",
				Options: &SpannerSequenceOptions{
					SequenceKind:     SpannerSequenceKindBitReversedPositive,
					StartWithCounter: wrapperspb.Int64(500),
				},
			},
			want:    "ALTER SEQUENCE `MySequence` SET OPTIONS (sequence_kind = 'bit_reversed_positive', start_with_counter = 500)",
			wantErr: false,
		},
		{
			name: "SpannerSequence.alterDdl.skipRangeMissingMin",
			fields: fields{
				Name: "MySequence",
				Options: &SpannerSequenceOptions{
					SequenceKind: SpannerSequenceKindBitReversedPositive,
					SkipRange: &SpannerSequenceSkipRange{
						Max: wrapperspb.Int64(5000000),
					},
				},
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "SpannerSequence.alterDdl.skipRangeMissingMax",
			fields: fields{
				Name: "MySequence",
				Options: &SpannerSequenceOptions{
					SequenceKind: SpannerSequenceKindBitReversedPositive,
					SkipRange: &SpannerSequenceSkipRange{
						Min: wrapperspb.Int64(1000),
					},
				},
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "SpannerSequence.alterDdl.skipRangeEmpty",
			fields: fields{
				Name: "MySequence",
				Options: &SpannerSequenceOptions{
					SequenceKind: SpannerSequenceKindBitReversedPositive,
					SkipRange:    &SpannerSequenceSkipRange{},
				},
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			t := &SpannerSequence{
				Name:    tt.fields.Name,
				Options: tt.fields.Options,
			}

			got, err := t.AlterDdl()
			if (err != nil) != tt.wantErr {
				t1.Errorf("alterDdl() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t1.Errorf("alterDdl() got = %v, want %v", got, tt.want)
			}
			t1.Logf("alterDdl() got = %v", got)
		})
	}
}

func Test_SpannerSequenceKindFromString(t *testing.T) {
	if got := SpannerSequenceKindFromString("bit_reversed_positive"); got != SpannerSequenceKindBitReversedPositive {
		t.Errorf("SpannerSequenceKindFromString(bit_reversed_positive) = %v, want SpannerSequenceKindBitReversedPositive", got)
	}
	// Unknown kinds fall back to the only supported kind.
	if got := SpannerSequenceKindFromString("unknown"); got != SpannerSequenceKindBitReversedPositive {
		t.Errorf("SpannerSequenceKindFromString(unknown) = %v, want SpannerSequenceKindBitReversedPositive", got)
	}
}

func Test_SpannerSequence_dropDdl(t1 *testing.T) {
	tests := []struct {
		name     string
		sequence *SpannerSequence
		want     string
		wantErr  bool
	}{
		{
			name:     "SpannerSequence.dropDdl",
			sequence: &SpannerSequence{Name: "MySequence"},
			want:     "DROP SEQUENCE `MySequence`",
			wantErr:  false,
		},
		{
			name:     "SpannerSequence.dropDdl.fullyQualifiedNameUsesLastSegment",
			sequence: &SpannerSequence{Name: "projects/my-project/instances/my-instance/databases/my-db/sequences/MySequence"},
			want:     "DROP SEQUENCE `MySequence`",
			wantErr:  false,
		},
		{
			name:     "SpannerSequence.dropDdl.nilReceiver",
			sequence: nil,
			want:     "",
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t1.Run(tt.name, func(t1 *testing.T) {
			got, err := tt.sequence.DropDdl()
			if (err != nil) != tt.wantErr {
				t1.Errorf("dropDdl() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t1.Errorf("dropDdl() got = %v, want %v", got, tt.want)
			}
		})
	}
}

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

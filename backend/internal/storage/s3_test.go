package storage

import "testing"

func TestCreateBucketInputLocationConstraint(t *testing.T) {
	tests := []struct {
		region    string
		wantValue string
	}{
		{region: "us-east-2", wantValue: "us-east-2"},
		{region: "eu-west-1", wantValue: "eu-west-1"},
		{region: "us-east-1"},
		{region: ""},
	}

	for _, tt := range tests {
		t.Run(tt.region, func(t *testing.T) {
			s := &Store{bucket: "photos", region: tt.region}
			input := s.createBucketInput()
			if input.Bucket == nil || *input.Bucket != "photos" {
				t.Fatalf("bucket = %v, want photos", input.Bucket)
			}
			if tt.wantValue == "" {
				if input.CreateBucketConfiguration != nil {
					t.Fatalf("location constraint = %q, want none", input.CreateBucketConfiguration.LocationConstraint)
				}
				return
			}
			if input.CreateBucketConfiguration == nil {
				t.Fatal("location constraint missing")
			}
			if got := string(input.CreateBucketConfiguration.LocationConstraint); got != tt.wantValue {
				t.Errorf("location constraint = %q, want %q", got, tt.wantValue)
			}
		})
	}
}

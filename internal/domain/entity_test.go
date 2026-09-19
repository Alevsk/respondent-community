package domain_test

import (
	"testing"

	"github.com/Alevsk/respondent/internal/domain"
)

func TestEntity_Validate(t *testing.T) {
	tests := []struct {
		name    string
		entity  domain.Entity
		wantErr bool
	}{
		{name: "valid entity", entity: domain.Entity{ID: "abc", LayerType: "adsb"}, wantErr: false},
		{name: "missing ID", entity: domain.Entity{LayerType: "adsb"}, wantErr: true},
		{name: "missing layer type", entity: domain.Entity{ID: "abc"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.entity.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

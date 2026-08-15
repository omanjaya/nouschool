package tenant

import (
	"context"
	"testing"
	"time"

	"github.com/omanjaya/nouschool/internal/platform/clock"
	"github.com/omanjaya/nouschool/internal/platform/httpx"
)

// fakeInterestRepo implementasi interestRepository in-memory (tanpa DB).
type fakeInterestRepo struct {
	leads  []InterestLead
	nextID int64
}

func (f *fakeInterestRepo) CreateInterestLead(ctx context.Context, in CreateInterestLeadInput) (InterestLead, error) {
	f.nextID++
	lead := InterestLead{ID: f.nextID, SchoolName: in.SchoolName, ContactName: in.ContactName, Phone: in.Phone, Email: in.Email, Note: in.Note}
	f.leads = append(f.leads, lead)
	return lead, nil
}

func (f *fakeInterestRepo) ListInterestLeads(ctx context.Context) ([]InterestLead, error) {
	return f.leads, nil
}

func validInterestInput(ip string) SubmitInterestInput {
	return SubmitInterestInput{SchoolName: "SMK Uji", ContactName: "Budi", Phone: "081234567890", ClientIP: ip}
}

// TestSubmitInterestRateLimitPerIP — docs tugas: "rate limit interest"
// (3/jam per IP).
func TestSubmitInterestRateLimitPerIP(t *testing.T) {
	repo := &fakeInterestRepo{}
	now := time.Date(2026, 8, 16, 3, 0, 0, 0, time.UTC)
	svc := newInterestServiceForTest(repo, 3, time.Hour, clock.Fixed{T: now})

	for i := 0; i < 3; i++ {
		if _, err := svc.SubmitInterest(context.Background(), validInterestInput("198.51.100.5")); err != nil {
			t.Fatalf("percobaan ke-%d seharusnya lolos: %v", i+1, err)
		}
	}
	// Percobaan ke-4 dari IP yang SAMA dalam jendela yang sama -> ditolak.
	if _, err := svc.SubmitInterest(context.Background(), validInterestInput("198.51.100.5")); err == nil {
		t.Fatal("percobaan ke-4 seharusnya kena rate limit")
	} else {
		de, ok := err.(*httpx.Error)
		if !ok {
			t.Fatalf("expected *httpx.Error, dapat %T", err)
		}
		if de.Status != 429 {
			t.Fatalf("expected status 429, dapat %d", de.Status)
		}
	}
	if len(repo.leads) != 3 {
		t.Fatalf("hanya 3 lead seharusnya tersimpan, dapat %d", len(repo.leads))
	}

	// IP LAIN tidak terpengaruh limit IP pertama.
	if _, err := svc.SubmitInterest(context.Background(), validInterestInput("203.0.113.9")); err != nil {
		t.Fatalf("IP lain seharusnya tidak kena limit: %v", err)
	}
}

func TestSubmitInterestValidation(t *testing.T) {
	repo := &fakeInterestRepo{}
	svc := newInterestServiceForTest(repo, 3, time.Hour, clock.Fixed{T: time.Now()})

	cases := []SubmitInterestInput{
		{SchoolName: "", ContactName: "Budi", Phone: "0812", ClientIP: "1.1.1.1"},
		{SchoolName: "SMK Uji", ContactName: "", Phone: "0812", ClientIP: "1.1.1.2"},
		{SchoolName: "SMK Uji", ContactName: "Budi", Phone: "", ClientIP: "1.1.1.3"},
	}
	for i, in := range cases {
		if _, err := svc.SubmitInterest(context.Background(), in); err == nil {
			t.Fatalf("case %d seharusnya validation error", i)
		}
	}
}

func TestListInterestReturnsSubmitted(t *testing.T) {
	repo := &fakeInterestRepo{}
	svc := newInterestServiceForTest(repo, 3, time.Hour, clock.Fixed{T: time.Now()})
	if _, err := svc.SubmitInterest(context.Background(), validInterestInput("198.51.100.5")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	items, err := svc.ListInterest(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].SchoolName != "SMK Uji" {
		t.Fatalf("list interest salah: %+v", items)
	}
}

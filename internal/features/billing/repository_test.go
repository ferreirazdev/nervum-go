package billing

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testPlanDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&BillingPlan{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestPlanRepository_ListActive_GetBySlug_Unit(t *testing.T) {
	db := testPlanDB(t)
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	repo := NewPlanRepository(db)
	ctx := context.Background()

	hidden := &BillingPlan{
		Slug:          "hidden",
		Name:          "Hidden",
		StripePriceID: "price_REPLACE_WITH_X",
		Active:        true,
		SortOrder:     0,
	}
	if err := db.Create(hidden).Error; err != nil {
		t.Fatalf("create hidden: %v", err)
	}

	visible := &BillingPlan{
		Slug:                 "pro",
		Name:                 "Pro",
		StripePriceID:        "price_123",
		Currency:             "usd",
		PriceInterval:        "month",
		Active:               true,
		SortOrder:            1,
		DisplayAmountCents:   ptr(9900),
	}
	if err := db.Create(visible).Error; err != nil {
		t.Fatalf("create visible: %v", err)
	}

	list, err := repo.ListActive(ctx)
	if err != nil {
		t.Fatalf("ListActive: %v", err)
	}
	if len(list) != 1 || list[0].Slug != "pro" {
		t.Fatalf("ListActive: got %#v", list)
	}

	got, err := repo.GetActiveBySlug(ctx, "pro")
	if err != nil {
		t.Fatalf("GetActiveBySlug: %v", err)
	}
	if got.ID == uuid.Nil || got.StripePriceID != "price_123" {
		t.Fatalf("GetActiveBySlug: %#v", got)
	}

	_, err = repo.GetActiveBySlug(ctx, "hidden")
	if err == nil {
		t.Fatal("expected error for REPLACE price id")
	}
}

func ptr(n int) *int { return &n }

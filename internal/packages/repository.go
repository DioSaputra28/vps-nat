package packages

import (
	"context"

	"github.com/DioSaputra28/vps-nat/internal/model"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, pkg *model.Package) error {
	return r.db.WithContext(ctx).Create(pkg).Error
}

func (r *Repository) FindAll(ctx context.Context, isActive *bool) ([]model.Package, error) {
	query := r.db.WithContext(ctx).Model(&model.Package{})
	if isActive != nil {
		query = query.Where("is_active = ?", *isActive)
	}

	var packages []model.Package
	err := query.Order("created_at DESC").Find(&packages).Error
	return packages, err
}

func (r *Repository) FindByID(ctx context.Context, id string) (*model.Package, error) {
	var pkg model.Package
	if err := r.db.WithContext(ctx).First(&pkg, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return &pkg, nil
}

func (r *Repository) Update(ctx context.Context, pkg *model.Package, updates map[string]any) error {
	return r.db.WithContext(ctx).Model(pkg).Updates(updates).Error
}

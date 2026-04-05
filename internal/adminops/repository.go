package adminops

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

type DashboardCounts struct {
	UsersTotal             int64
	UsersActive            int64
	ServicesActive         int64
	ContainersRunning      int64
	ContainersStopped      int64
	ProvisioningPending    int64
	AllocatedCPUCores      int64
	AllocatedRAMMB         int64
	AllocatedDiskGB        int64
	GrossRevenue           int64
	RefundedAmount         int64
	ServerCostTotal        int64
	ServerCostCount        int64
	LastServerCostRecorded *time.Time
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CountUsers(ctx context.Context) (total int64, active int64, err error) {
	if err = r.db.WithContext(ctx).Model(&model.User{}).Count(&total).Error; err != nil {
		return 0, 0, err
	}
	if err = r.db.WithContext(ctx).Model(&model.User{}).Where("status = ?", "active").Count(&active).Error; err != nil {
		return 0, 0, err
	}
	return total, active, nil
}

func (r *Repository) CountServices(ctx context.Context) (activeServices int64, runningContainers int64, stoppedContainers int64, provisioningPending int64, err error) {
	if err = r.db.WithContext(ctx).Model(&model.Service{}).Where("status = ?", "active").Count(&activeServices).Error; err != nil {
		return 0, 0, 0, 0, err
	}
	if err = r.db.WithContext(ctx).Model(&model.ServiceInstance{}).Where("status = ?", "running").Count(&runningContainers).Error; err != nil {
		return 0, 0, 0, 0, err
	}
	if err = r.db.WithContext(ctx).Model(&model.ServiceInstance{}).Where("status = ?", "stopped").Count(&stoppedContainers).Error; err != nil {
		return 0, 0, 0, 0, err
	}
	if err = r.db.WithContext(ctx).Model(&model.ProvisioningJob{}).
		Where("job_type = ? AND status IN ?", "provision", []string{"queued", "running"}).
		Count(&provisioningPending).Error; err != nil {
		return 0, 0, 0, 0, err
	}
	return activeServices, runningContainers, stoppedContainers, provisioningPending, nil
}

func (r *Repository) AllocatedResources(ctx context.Context) (cpuCores int64, ramMB int64, diskGB int64, err error) {
	type row struct {
		CPU  int64 `gorm:"column:cpu"`
		RAM  int64 `gorm:"column:ram"`
		Disk int64 `gorm:"column:disk"`
	}

	var result row
	err = r.db.WithContext(ctx).Model(&model.Service{}).
		Where("status = ?", "active").
		Select("COALESCE(SUM(cpu_snapshot), 0) AS cpu, COALESCE(SUM(ram_mb_snapshot), 0) AS ram, COALESCE(SUM(disk_gb_snapshot), 0) AS disk").
		Scan(&result).Error
	if err != nil {
		return 0, 0, 0, err
	}

	return result.CPU, result.RAM, result.Disk, nil
}

func (r *Repository) RevenueSummary(ctx context.Context) (grossRevenue int64, refundedAmount int64, err error) {
	type grossRow struct {
		Gross int64 `gorm:"column:gross"`
	}
	type refundRow struct {
		Refunded int64 `gorm:"column:refunded"`
	}

	var gross grossRow
	err = r.db.WithContext(ctx).Model(&model.Order{}).
		Where("order_type IN ? AND payment_status = ?", []string{"purchase", "renewal", "upgrade"}, "paid").
		Select("COALESCE(SUM(total_amount), 0) AS gross").
		Scan(&gross).Error
	if err != nil {
		return 0, 0, err
	}

	var refund refundRow
	err = r.db.WithContext(ctx).Model(&model.WalletTransaction{}).
		Where("transaction_type = ? AND direction = ?", "refund", "credit").
		Select("COALESCE(SUM(amount), 0) AS refunded").
		Scan(&refund).Error
	if err != nil {
		return 0, 0, err
	}

	return gross.Gross, refund.Refunded, nil
}

func (r *Repository) ServerCostSummary(ctx context.Context) (total int64, count int64, lastRecordedAt *time.Time, err error) {
	var totalAmount int64
	var countRows int64
	var lastRaw any

	row := r.db.WithContext(ctx).Model(&model.ServerCost{}).
		Select("COALESCE(SUM(purchase_cost), 0) AS total, COUNT(*) AS count, MAX(recorded_at) AS last_recorded_at").
		Row()
	if err := row.Scan(&totalAmount, &countRows, &lastRaw); err != nil {
		return 0, 0, nil, err
	}

	lastRecordedAt, err = normalizeRecordedAt(lastRaw)
	if err != nil {
		return 0, 0, nil, err
	}

	return totalAmount, countRows, lastRecordedAt, nil
}

func (r *Repository) ListServerCosts(ctx context.Context) ([]model.ServerCost, error) {
	var items []model.ServerCost
	if err := r.db.WithContext(ctx).
		Preload("Node").
		Preload("CreatedByAdmin").
		Order("recorded_at DESC, created_at DESC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) FindUserWithWalletByID(ctx context.Context, userID string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).
		Preload("Wallet").
		First(&user, "id = ?", strings.TrimSpace(userID)).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) FindAdminByID(ctx context.Context, adminID string) (*model.AdminUser, error) {
	var admin model.AdminUser
	if err := r.db.WithContext(ctx).First(&admin, "id = ?", strings.TrimSpace(adminID)).Error; err != nil {
		return nil, err
	}
	return &admin, nil
}

func (r *Repository) CreateServerCost(ctx context.Context, cost *model.ServerCost) error {
	return r.db.WithContext(ctx).Create(cost).Error
}

func (r *Repository) CreateWalletTransaction(ctx context.Context, tx *model.WalletTransaction) error {
	return r.db.WithContext(ctx).Create(tx).Error
}

func (r *Repository) CreateActivityLog(ctx context.Context, logEntry *model.ActivityLog) error {
	return r.db.WithContext(ctx).Create(logEntry).Error
}

func (r *Repository) ListActivityLogs(ctx context.Context, input ListActivityLogsInput) ([]model.ActivityLog, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.ActivityLog{})
	if actorType := strings.TrimSpace(input.ActorType); actorType != "" {
		query = query.Where("actor_type = ?", actorType)
	}
	if action := strings.TrimSpace(input.Action); action != "" {
		query = query.Where("action = ?", action)
	}
	if targetType := strings.TrimSpace(input.TargetType); targetType != "" {
		query = query.Where("target_type = ?", targetType)
	}
	if targetID := strings.TrimSpace(input.TargetID); targetID != "" {
		query = query.Where("target_id = ?", targetID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.ActivityLog
	if err := query.
		Order("created_at DESC").
		Limit(input.Limit).
		Offset((input.Page - 1) * input.Limit).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *Repository) FindAdminsByIDs(ctx context.Context, ids []string) ([]model.AdminUser, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var admins []model.AdminUser
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&admins).Error; err != nil {
		return nil, err
	}
	return admins, nil
}

func (r *Repository) FindUsersByIDs(ctx context.Context, ids []string) ([]model.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var users []model.User
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *Repository) CountOpenAlerts(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.ResourceAlert{}).Where("status = ?", "open").Count(&count).Error
	return count, err
}

func (r *Repository) ListLatestOpenAlerts(ctx context.Context, limit int) ([]model.ResourceAlert, error) {
	if limit <= 0 {
		limit = 10
	}

	var alerts []model.ResourceAlert
	if err := r.db.WithContext(ctx).
		Model(&model.ResourceAlert{}).
		Preload("Service.OwnerUser").
		Preload("Service.Instance").
		Preload("Node").
		Where("status = ?", "open").
		Order("opened_at DESC").
		Limit(limit).
		Find(&alerts).Error; err != nil {
		return nil, err
	}
	return alerts, nil
}

func (r *Repository) ListAlerts(ctx context.Context, input ListAlertsInput) ([]model.ResourceAlert, int64, error) {
	query := r.db.WithContext(ctx).
		Model(&model.ResourceAlert{}).
		Preload("Service.OwnerUser").
		Preload("Service.Instance").
		Preload("Node")

	if status := strings.TrimSpace(input.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	if alertType := strings.TrimSpace(input.AlertType); alertType != "" {
		query = query.Where("alert_type = ?", alertType)
	}
	if serviceID := strings.TrimSpace(input.ServiceID); serviceID != "" {
		query = query.Where("service_id = ?", serviceID)
	}
	if nodeID := strings.TrimSpace(input.NodeID); nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.ResourceAlert
	if err := query.
		Order("opened_at DESC").
		Limit(input.Limit).
		Offset((input.Page - 1) * input.Limit).
		Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *Repository) ListMonitoredInstances(ctx context.Context) ([]model.ServiceInstance, error) {
	var items []model.ServiceInstance
	if err := r.db.WithContext(ctx).
		Model(&model.ServiceInstance{}).
		Preload("Node").
		Preload("Service").
		Preload("Service.OwnerUser").
		Where("incus_instance_name <> ''").
		Where("status IN ?", []string{"running", "stopped", "suspended"}).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) FindOpenAlertsForServices(ctx context.Context, serviceIDs []string) ([]model.ResourceAlert, error) {
	if len(serviceIDs) == 0 {
		return nil, nil
	}

	var items []model.ResourceAlert
	if err := r.db.WithContext(ctx).
		Model(&model.ResourceAlert{}).
		Where("status = ? AND service_id IN ?", "open", serviceIDs).
		Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func normalizeRecordedAt(value any) (*time.Time, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case time.Time:
		t := v.UTC()
		return &t, nil
	case *time.Time:
		if v == nil {
			return nil, nil
		}
		t := v.UTC()
		return &t, nil
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, nil
		}
		parsed, err := parseTimeString(v)
		if err != nil {
			return nil, err
		}
		return &parsed, nil
	case []byte:
		if len(v) == 0 {
			return nil, nil
		}
		parsed, err := parseTimeString(string(v))
		if err != nil {
			return nil, err
		}
		return &parsed, nil
	case sql.NullString:
		if !v.Valid || strings.TrimSpace(v.String) == "" {
			return nil, nil
		}
		parsed, err := parseTimeString(v.String)
		if err != nil {
			return nil, err
		}
		return &parsed, nil
	default:
		return nil, nil
	}
}

func parseTimeString(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999999999",
	}

	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse time value %q", value)
}

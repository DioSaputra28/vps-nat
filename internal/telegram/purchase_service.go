package telegram

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/DioSaputra28/vps-nat/internal/model"
	telegramservice "github.com/DioSaputra28/vps-nat/internal/telegram/service"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var hostnamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

type purchasePortTemplate struct {
	mappingType string
	protocol    string
	targetPort  int
	description string
}

var defaultPurchasePortBundle = []purchasePortTemplate{
	{mappingType: "ssh", protocol: "tcp", targetPort: 22, description: "SSH"},
	{mappingType: "custom", protocol: "tcp", targetPort: 3000, description: "App 3000"},
	{mappingType: "custom", protocol: "tcp", targetPort: 8000, description: "App 8000"},
	{mappingType: "custom", protocol: "tcp", targetPort: 8080, description: "App 8080"},
	{mappingType: "custom", protocol: "tcp", targetPort: 8888, description: "App 8888"},
	{mappingType: "custom", protocol: "tcp", targetPort: 25565, description: "Game 25565"},
	{mappingType: "custom", protocol: "udp", targetPort: 19132, description: "Game 19132"},
}

func (s *Service) BuyVPSSubmit(ctx context.Context, input BuyVPSSubmitInput) (*BuyVPSSubmitResult, error) {
	log.Printf("[purchase][submit] telegram_id=%d package_id=%s hostname=%q payment_method=%s image_alias=%q", input.TelegramID, input.PackageID, input.Hostname, input.PaymentMethod, input.ImageAlias)
	if input.TelegramID <= 0 {
		return nil, ErrInvalidTelegramUser
	}

	hostname, err := normalizeHostname(input.Hostname)
	if err != nil {
		log.Printf("[purchase][submit] hostname normalization failed hostname=%q: %v", input.Hostname, err)
		return nil, err
	}

	if !telegramservice.ValidatePaymentMethod(input.PaymentMethod) {
		log.Printf("[purchase][submit] unsupported payment method=%q", input.PaymentMethod)
		return nil, ErrUnsupportedPayment
	}
	if _, ok := findImageOption(input.ImageAlias); !ok {
		log.Printf("[purchase][submit] invalid image alias=%q", input.ImageAlias)
		return nil, ErrInvalidActionRequest
	}

	user, err := s.repo.FindUserByTelegramID(ctx, input.TelegramID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[purchase][submit] telegram user not found telegram_id=%d", input.TelegramID)
			return nil, ErrTelegramUserNotFound
		}
		return nil, err
	}
	log.Printf("[purchase][submit] user resolved user_id=%s wallet_present=%t", user.ID, user.Wallet != nil)

	pkg, err := s.repo.FindPackageByID(ctx, input.PackageID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[purchase][submit] package not found package_id=%s", input.PackageID)
			return nil, ErrPackageNotFound
		}
		return nil, err
	}
	if !pkg.IsActive {
		log.Printf("[purchase][submit] package inactive package_id=%s", input.PackageID)
		return nil, ErrPackageNotFound
	}
	log.Printf("[purchase][submit] package resolved package_id=%s name=%s price=%d duration_days=%d", pkg.ID, pkg.Name, pkg.Price, pkg.DurationDays)

	conflict, err := s.repo.HasHostnameConflict(ctx, hostname)
	if err != nil {
		return nil, err
	}
	if conflict {
		log.Printf("[purchase][submit] hostname conflict in database hostname=%s", hostname)
		return nil, ErrHostnameAlreadyExists
	}
	if s.purchaseProvisioner != nil {
		exists, err := s.purchaseProvisioner.HostnameExists(ctx, hostname)
		if err != nil {
			return nil, err
		}
		if exists {
			log.Printf("[purchase][submit] hostname conflict in incus hostname=%s", hostname)
			return nil, ErrHostnameAlreadyExists
		}
	}

	node, err := s.repo.FindActiveNode(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[purchase][submit] no active node available")
			return nil, ErrActiveNodeNotFound
		}
		return nil, err
	}
	log.Printf("[purchase][submit] active node resolved node_id=%s public_ip=%s", node.ID, node.PublicIP)

	switch input.PaymentMethod {
	case "wallet":
		return s.submitWalletPurchase(ctx, user, pkg, node, hostname, input.ImageAlias)
	case "qris":
		return s.submitQrisPurchase(ctx, user, pkg, hostname, input.ImageAlias)
	default:
		return nil, ErrUnsupportedPayment
	}
}

func (s *Service) BuyVPSStatus(ctx context.Context, input BuyVPSStatusInput) (*BuyVPSStatusResult, error) {
	log.Printf("[purchase][status] telegram_id=%d order_id=%s", input.TelegramID, input.OrderID)
	if input.TelegramID <= 0 || strings.TrimSpace(input.OrderID) == "" {
		return nil, ErrInvalidActionRequest
	}

	order, err := s.repo.FindPurchaseOrderForUser(ctx, input.TelegramID, input.OrderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}

	payment := latestPayment(order.Payments)
	status := purchaseStatus(order, payment)
	result := &BuyVPSStatusResult{
		OrderID: order.ID,
		Status:  status,
	}
	log.Printf("[purchase][status] order_id=%s resolved status=%s", order.ID, status)

	if payment != nil && status == "awaiting_payment" {
		result.Payment = paymentPayloadFromModel(payment)
	}

	if order.Service != nil && order.Service.Instance != nil && order.Service.Status == "active" {
		password := ""
		if payment != nil {
			password = extractOneTimePassword(payment)
		}
		result.Status = "active"
		result.Service = buildServicePayload(order, order.Service, order.Service.Instance, order.Service.PortMappings, password)
		if payment != nil && password != "" {
			clearOneTimePassword(payment.ProviderPayload)
			_ = s.repo.UpdatePaymentProviderPayload(ctx, payment.ID, payment.ProviderPayload)
		}
	}

	if order.Status == "failed" {
		msg := "order failed"
		result.Error = &msg
	}

	return result, nil
}

func (s *Service) HandlePakasirWebhook(ctx context.Context, input PakasirWebhookInput) error {
	log.Printf("[purchase][webhook][pakasir] order_id=%s status=%s amount=%d method=%s", input.OrderID, input.Status, input.Amount, input.PaymentMethod)
	if strings.TrimSpace(input.OrderID) == "" {
		return ErrInvalidActionRequest
	}
	if !strings.EqualFold(strings.TrimSpace(input.Status), "completed") {
		return nil
	}

	order, err := s.repo.FindPurchaseOrderByID(ctx, input.OrderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrOrderNotFound
		}
		return err
	}
	if order.Status == "completed" || order.Status == "failed" || order.Status == "canceled" {
		return nil
	}

	payment := latestPayment(order.Payments)
	if payment == nil {
		return ErrOrderNotFound
	}
	if s.paymentGateway == nil {
		return ErrPaymentVerification
	}

	verified, err := s.paymentGateway.VerifyTransaction(ctx, PakasirVerifyTransactionRequest{
		OrderID: order.ID,
		Amount:  order.TotalAmount,
	})
	if err != nil {
		log.Printf("[purchase][webhook][pakasir] verification call failed order_id=%s: %v", order.ID, err)
		return err
	}
	if !strings.EqualFold(verified.Status, "completed") || verified.Amount != order.TotalAmount || !strings.EqualFold(verified.PaymentMethod, "qris") {
		log.Printf("[purchase][webhook][pakasir] verification rejected order_id=%s verified_status=%s verified_amount=%d verified_method=%s", order.ID, verified.Status, verified.Amount, verified.PaymentMethod)
		return ErrPaymentVerification
	}
	log.Printf("[purchase][webhook][pakasir] verification accepted order_id=%s", order.ID)

	now := time.Now().UTC()
	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Order{}).
			Where("id = ?", order.ID).
			Updates(map[string]any{
				"status":         "paid",
				"payment_status": "paid",
				"paid_at":        now,
				"updated_at":     now,
			}).Error; err != nil {
			return err
		}
		payload := payment.ProviderPayload
		if payload == nil {
			payload = map[string]any{}
		}
		payload["verified_transaction"] = verified.Raw
		return tx.Model(&model.Payment{}).
			Where("id = ?", payment.ID).
			Updates(&model.Payment{
				Status:          "paid",
				PaidAt:          &now,
				ProviderPayload: payload,
				UpdatedAt:       now,
			}).Error
	}); err != nil {
		return err
	}

	order.PaymentStatus = "paid"
	order.Status = "paid"
	order.PaidAt = &now
	payment.Status = "paid"
	payment.PaidAt = &now
	if payment.ProviderPayload == nil {
		payment.ProviderPayload = map[string]any{}
	}
	payment.ProviderPayload["verified_transaction"] = verified.Raw

	hostname, _ := stringValue(payment.ProviderPayload["hostname"])
	imageAlias, _ := stringValue(payment.ProviderPayload["image_alias"])
	if hostname == "" || imageAlias == "" {
		return ErrInvalidActionRequest
	}

	node, err := s.repo.FindActiveNode(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrActiveNodeNotFound
		}
		return err
	}

	_, err = s.provisionPaidPurchase(ctx, provisionPaidPurchaseInput{
		User:       order.User,
		Wallet:     order.User.Wallet,
		Order:      order,
		Payment:    payment,
		Node:       node,
		Hostname:   hostname,
		ImageAlias: imageAlias,
		ActorType:  actorTypeSystem,
		ActorID:    nil,
	})
	return err
}

func (s *Service) submitWalletPurchase(ctx context.Context, user *model.User, pkg *model.Package, node *model.Node, hostname string, imageAlias string) (*BuyVPSSubmitResult, error) {
	if user.Wallet == nil || user.Wallet.Balance < pkg.Price {
		var balance int64
		if user.Wallet != nil {
			balance = user.Wallet.Balance
		}
		log.Printf("[purchase][wallet] insufficient balance user_id=%s balance=%d required=%d", user.ID, balance, pkg.Price)
		return nil, ErrInsufficientBalance
	}
	if s.purchaseProvisioner == nil {
		return nil, ErrIncusUnavailable
	}
	log.Printf("[purchase][wallet] creating paid order user_id=%s hostname=%s package_id=%s amount=%d", user.ID, hostname, pkg.ID, pkg.Price)

	now := time.Now().UTC()
	order := telegramservice.BuildOrder(user.ID, pkg, "purchase", nil, pkg.Price, &imageAlias, now)
	order.Status = "processing"
	order.PaymentStatus = "paid"
	order.PaymentMethod = strPtr("wallet")
	order.PaidAt = &now

	payment := telegramservice.BuildPayment(&order.ID, "order", "wallet", pkg.Price, now)
	payment.Status = "paid"
	payment.PaidAt = &now
	payment.ProviderPayload = map[string]any{
		"hostname":    hostname,
		"image_alias": imageAlias,
	}

	balanceAfter := user.Wallet.Balance - pkg.Price
	sourceOrderID := order.ID
	note := "purchase payment"
	walletTxn := &model.WalletTransaction{
		ID:              uuid.NewString(),
		WalletID:        user.Wallet.ID,
		UserID:          user.ID,
		Direction:       "debit",
		TransactionType: "payment",
		Amount:          pkg.Price,
		BalanceBefore:   user.Wallet.Balance,
		BalanceAfter:    balanceAfter,
		SourceType:      "order",
		SourceID:        &sourceOrderID,
		Note:            &note,
		CreatedAt:       now,
	}

	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		if err := tx.Create(&payment).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Wallet{}).Where("id = ?", user.Wallet.ID).Update("balance", balanceAfter).Error; err != nil {
			return err
		}
		return tx.Create(walletTxn).Error
	}); err != nil {
		return nil, err
	}
	log.Printf("[purchase][wallet] order created order_id=%s payment_id=%s wallet_balance_after=%d", order.ID, payment.ID, balanceAfter)

	user.Wallet.Balance = balanceAfter
	access, err := s.provisionPaidPurchase(ctx, provisionPaidPurchaseInput{
		User:       user,
		Wallet:     user.Wallet,
		Order:      &order,
		Payment:    &payment,
		Node:       node,
		Hostname:   hostname,
		ImageAlias: imageAlias,
		ActorType:  actorTypeTelegramUser,
		ActorID:    &user.ID,
	})
	if err != nil {
		return nil, err
	}

	return &BuyVPSSubmitResult{
		OrderID:   order.ID,
		PaymentID: payment.ID,
		Status:    "active",
		Service:   access,
	}, nil
}

func (s *Service) submitQrisPurchase(ctx context.Context, user *model.User, pkg *model.Package, hostname string, imageAlias string) (*BuyVPSSubmitResult, error) {
	if s.paymentGateway == nil {
		return nil, ErrUnsupportedPayment
	}
	log.Printf("[purchase][qris] creating pending order user_id=%s hostname=%s package_id=%s amount=%d", user.ID, hostname, pkg.ID, pkg.Price)

	now := time.Now().UTC()
	order := telegramservice.BuildOrder(user.ID, pkg, "purchase", nil, pkg.Price, &imageAlias, now)
	order.Status = "awaiting_payment"
	order.PaymentStatus = "pending"
	order.PaymentMethod = strPtr("qris")

	transaction, err := s.paymentGateway.CreateQris(ctx, PakasirCreateTransactionRequest{
		OrderID: order.ID,
		Amount:  pkg.Price,
		Method:  "qris",
	})
	if err != nil {
		log.Printf("[purchase][qris] create transaction failed hostname=%s: %v", hostname, err)
		return nil, err
	}
	log.Printf("[purchase][qris] transaction created order_id=%s amount=%d fee=%d total=%d", order.ID, transaction.Amount, transaction.Fee, transaction.TotalPayment)

	payment := telegramservice.BuildPayment(&order.ID, "order", "qris", pkg.Price, now)
	payment.Provider = strPtr("pakasir")
	payment.ProviderRef = &order.ID
	payment.ProviderPayload = map[string]any{
		"hostname":      hostname,
		"image_alias":   imageAlias,
		"provider_data": transaction.Raw,
		"fee":           transaction.Fee,
		"total_payment": transaction.TotalPayment,
		"qr_string":     transaction.QRString,
	}
	payment.ExpiredAt = &transaction.ExpiredAt
	payment.Status = "pending"

	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		return tx.Create(&payment).Error
	}); err != nil {
		return nil, err
	}

	return &BuyVPSSubmitResult{
		OrderID:   order.ID,
		PaymentID: payment.ID,
		Status:    "awaiting_payment",
		Payment: &BuyVPSPaymentPayload{
			Provider:      transaction.Provider,
			PaymentMethod: "qris",
			Amount:        transaction.Amount,
			Fee:           transaction.Fee,
			TotalPayment:  transaction.TotalPayment,
			QRString:      transaction.QRString,
			ExpiredAt:     &transaction.ExpiredAt,
		},
	}, nil
}

type provisionPaidPurchaseInput struct {
	User       *model.User
	Wallet     *model.Wallet
	Order      *model.Order
	Payment    *model.Payment
	Node       *model.Node
	Hostname   string
	ImageAlias string
	ActorType  string
	ActorID    *string
}

func (s *Service) provisionPaidPurchase(ctx context.Context, input provisionPaidPurchaseInput) (*BuyVPSServicePayload, error) {
	if s.purchaseProvisioner == nil {
		return nil, ErrIncusUnavailable
	}
	log.Printf("[purchase][provision] order_id=%s hostname=%s payment_method=%s node_public_ip=%s", input.Order.ID, input.Hostname, input.Payment.Method, input.Node.PublicIP)

	var lastErr error
	var provisioned *PurchaseProvisionResult
	attempts := 0

	for attempts = 1; attempts <= 3; attempts++ {
		log.Printf("[purchase][provision] order_id=%s hostname=%s attempt=%d started", input.Order.ID, input.Hostname, attempts)
		mappings, err := s.allocatePortMappings(ctx, input.Node.PublicIP)
		if err != nil {
			log.Printf("[purchase][provision] order_id=%s hostname=%s attempt=%d port allocation failed: %v", input.Order.ID, input.Hostname, attempts, err)
			return nil, err
		}
		log.Printf("[purchase][provision] order_id=%s hostname=%s attempt=%d port allocation completed mapping_count=%d", input.Order.ID, input.Hostname, attempts, len(mappings))

		provisioned, err = s.purchaseProvisioner.Provision(ctx, PurchaseProvisionRequest{
			OrderID:      input.Order.ID,
			Hostname:     input.Hostname,
			ImageAlias:   input.ImageAlias,
			Node:         *input.Node,
			Package:      packagePayloadFromOrder(input.Order),
			PortMappings: mappings,
		})
		if err == nil {
			log.Printf("[purchase][provision] order_id=%s hostname=%s attempt=%d succeeded", input.Order.ID, input.Hostname, attempts)
			break
		}

		lastErr = err
		log.Printf("[purchase][provision] order_id=%s hostname=%s attempt=%d failed: %v", input.Order.ID, input.Hostname, attempts, err)
	}

	if provisioned == nil {
		log.Printf("[purchase][provision] order_id=%s hostname=%s exhausted retries attempts=%d", input.Order.ID, input.Hostname, attempts-1)
		if err := s.persistPurchaseFailure(ctx, input, attempts-1, lastErr); err != nil {
			return nil, err
		}
		s.notifyProvisionFailure(ctx, input, lastErr)
		return nil, ErrProvisioningFailed
	}

	return s.persistPurchaseSuccess(ctx, input, attempts, provisioned)
}

func (s *Service) allocatePortMappings(ctx context.Context, publicIP string) ([]RequestedPortMapping, error) {
	usedTCP := map[int]struct{}{}
	usedUDP := map[int]struct{}{}
	mappings := make([]RequestedPortMapping, 0, len(defaultPurchasePortBundle))

	for _, item := range defaultPurchasePortBundle {
		var excluded map[int]struct{}
		if item.protocol == "udp" {
			excluded = usedUDP
		} else {
			excluded = usedTCP
		}

		port, err := s.repo.FindNextAvailablePublicPort(ctx, publicIP, item.protocol, excluded)
		if err != nil {
			return nil, err
		}
		excluded[port] = struct{}{}
		mappings = append(mappings, RequestedPortMapping{
			MappingType: item.mappingType,
			Protocol:    item.protocol,
			TargetPort:  item.targetPort,
			PublicPort:  port,
			Description: item.description,
		})
	}

	return mappings, nil
}

func (s *Service) persistPurchaseSuccess(ctx context.Context, input provisionPaidPurchaseInput, attempts int, result *PurchaseProvisionResult) (*BuyVPSServicePayload, error) {
	now := time.Now().UTC()
	log.Printf("[purchase][persist-success] order_id=%s hostname=%s attempts=%d service_status=active public_ip=%s private_ip=%s", input.Order.ID, input.Hostname, attempts, result.PublicIP, result.PrivateIP)
	expiresAt := now.Add(time.Duration(input.Order.DurationDaysSnapshot) * 24 * time.Hour)
	serviceID := uuid.NewString()
	containerID := uuid.NewString()

	service := &model.Service{
		ID:                  serviceID,
		OrderID:             input.Order.ID,
		OwnerUserID:         input.User.ID,
		CurrentPackageID:    input.Order.PackageID,
		Status:              "active",
		BillingCycleDays:    input.Order.DurationDaysSnapshot,
		PackageNameSnapshot: input.Order.PackageNameSnapshot,
		CPUSnapshot:         input.Order.CPUSnapshot,
		RAMMBSnapshot:       input.Order.RAMMBSnapshot,
		DiskGBSnapshot:      input.Order.DiskGBSnapshot,
		PriceSnapshot:       input.Order.PriceSnapshot,
		StartedAt:           &now,
		ExpiresAt:           &expiresAt,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	instance := &model.ServiceInstance{
		ID:                   containerID,
		ServiceID:            service.ID,
		NodeID:               input.Node.ID,
		IncusInstanceName:    result.Hostname,
		ImageAlias:           input.ImageAlias,
		OSFamily:             result.OSFamily,
		InternalIP:           strPtr(result.PrivateIP),
		MainPublicIP:         strPtr(result.PublicIP),
		SSHPort:              intPtr(result.SSHPort),
		Status:               "running",
		LastIncusOperationID: strPtr(result.OperationID),
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	portMappings := make([]model.ServicePortMapping, 0, len(result.PortMappings))
	for _, mapping := range result.PortMappings {
		portMappings = append(portMappings, model.ServicePortMapping{
			ID:          uuid.NewString(),
			ServiceID:   service.ID,
			MappingType: mapping.MappingType,
			PublicIP:    mapping.PublicIP,
			PublicPort:  mapping.PublicPort,
			Protocol:    mapping.Protocol,
			TargetIP:    mapping.TargetIP,
			TargetPort:  mapping.TargetPort,
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}

	job := newProvisioningJob(service.ID, &input.Order.ID, "success", attempts, input.ActorType, input.ActorID, result.OperationID, nil, map[string]any{
		"hostname": input.Hostname,
		"attempts": attempts,
	})
	event := &model.ServiceEvent{
		ID:        uuid.NewString(),
		ServiceID: service.ID,
		EventType: "service_created",
		ActorType: input.ActorType,
		ActorID:   input.ActorID,
		Summary:   fmt.Sprintf("service %s berhasil dibuat", input.Hostname),
		Payload: map[string]any{
			"order_id":   input.Order.ID,
			"container":  input.Hostname,
			"public_ip":  result.PublicIP,
			"private_ip": result.PrivateIP,
		},
		CreatedAt: now,
	}

	if err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Order{}).
			Where("id = ?", input.Order.ID).
			Updates(map[string]any{
				"status":         "completed",
				"payment_status": input.Order.PaymentStatus,
				"updated_at":     now,
			}).Error; err != nil {
			return err
		}
		if err := tx.Create(service).Error; err != nil {
			return err
		}
		if err := tx.Create(instance).Error; err != nil {
			return err
		}
		if len(portMappings) > 0 {
			if err := tx.Create(&portMappings).Error; err != nil {
				return err
			}
		}
		if input.Payment != nil && input.Payment.Method == "qris" {
			payload := input.Payment.ProviderPayload
			if payload == nil {
				payload = map[string]any{}
			}
			payload["initial_ssh_password"] = result.SSHPassword
			payload["credentials_delivered"] = false
			if err := tx.Model(&model.Payment{}).
				Where("id = ?", input.Payment.ID).
				Updates(&model.Payment{
					ProviderPayload: payload,
					UpdatedAt:       now,
				}).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(job).Error; err != nil {
			return err
		}
		return tx.Create(event).Error
	}); err != nil {
		return nil, err
	}

	password := result.SSHPassword
	if input.Payment != nil && input.Payment.Method == "qris" {
		password = ""
	}

	return buildServicePayload(input.Order, service, instance, portMappings, password), nil
}

func (s *Service) persistPurchaseFailure(ctx context.Context, input provisionPaidPurchaseInput, attempts int, lastErr error) error {
	now := time.Now().UTC()
	log.Printf("[purchase][persist-failure] order_id=%s hostname=%s attempts=%d last_error=%v", input.Order.ID, input.Hostname, attempts, lastErr)
	var errMsg *string
	if lastErr != nil {
		msg := lastErr.Error()
		errMsg = &msg
	}
	job := newProvisioningJob("", &input.Order.ID, "failed", attempts, input.ActorType, input.ActorID, "", errMsg, map[string]any{
		"hostname": input.Hostname,
		"attempts": attempts,
	})

	return s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Order{}).
			Where("id = ?", input.Order.ID).
			Updates(map[string]any{
				"status":         "failed",
				"payment_status": "refunded",
				"failed_at":      now,
				"updated_at":     now,
			}).Error; err != nil {
			return err
		}

		if input.Wallet != nil {
			balanceBefore := input.Wallet.Balance
			balanceAfter := balanceBefore + input.Order.TotalAmount
			if err := tx.Model(&model.Wallet{}).Where("id = ?", input.Wallet.ID).Update("balance", balanceAfter).Error; err != nil {
				return err
			}
			sourceOrderID := input.Order.ID
			note := "purchase provisioning failure compensation"
			walletTxn := &model.WalletTransaction{
				ID:              uuid.NewString(),
				WalletID:        input.Wallet.ID,
				UserID:          input.User.ID,
				Direction:       "credit",
				TransactionType: "refund",
				Amount:          input.Order.TotalAmount,
				BalanceBefore:   balanceBefore,
				BalanceAfter:    balanceAfter,
				SourceType:      "order",
				SourceID:        &sourceOrderID,
				Note:            &note,
				CreatedAt:       now,
			}
			if err := tx.Create(walletTxn).Error; err != nil {
				return err
			}
		}

		return tx.Create(job).Error
	})
}

func (s *Service) notifyProvisionFailure(ctx context.Context, input provisionPaidPurchaseInput, lastErr error) {
	if s.adminNotifier == nil {
		log.Printf("[purchase][notify-failure] order_id=%s hostname=%s notifier not configured", input.Order.ID, input.Hostname)
		return
	}

	message := fmt.Sprintf("Provisioning failed after 3 attempts\nOrder: %s\nHostname: %s\nUser: %d\nError: %v", input.Order.ID, input.Hostname, input.User.TelegramID, lastErr)
	log.Printf("[purchase][notify-failure] order_id=%s hostname=%s sending admin alert", input.Order.ID, input.Hostname)
	_ = s.adminNotifier.NotifyProvisionFailure(ctx, message)
}

func normalizeHostname(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if !hostnamePattern.MatchString(normalized) {
		return "", ErrInvalidActionRequest
	}
	return normalized, nil
}

func packagePayloadFromOrder(order *model.Order) BuyVPSPackagePayload {
	return BuyVPSPackagePayload{
		ID:           order.PackageID,
		Name:         order.PackageNameSnapshot,
		CPU:          order.CPUSnapshot,
		RAMMB:        order.RAMMBSnapshot,
		DiskGB:       order.DiskGBSnapshot,
		Price:        order.PriceSnapshot,
		DurationDays: order.DurationDaysSnapshot,
	}
}

func buildServicePayload(order *model.Order, service *model.Service, instance *model.ServiceInstance, mappings []model.ServicePortMapping, password string) *BuyVPSServicePayload {
	payload := &BuyVPSServicePayload{
		OrderID:      order.ID,
		ServiceID:    service.ID,
		ContainerID:  instance.ID,
		Package:      packagePayloadFromOrder(order),
		Hostname:     instance.IncusInstanceName,
		PublicIP:     derefString(instance.MainPublicIP),
		PrivateIP:    derefString(instance.InternalIP),
		SSHUsername:  "root",
		SSHPassword:  password,
		SSHPort:      derefInt(instance.SSHPort),
		ExpiresAt:    service.ExpiresAt,
		PortMappings: make([]BuyVPSPortMappingPayload, 0, len(mappings)),
	}
	for _, mapping := range mappings {
		payload.PortMappings = append(payload.PortMappings, BuyVPSPortMappingPayload{
			PublicIP:    mapping.PublicIP,
			PublicPort:  mapping.PublicPort,
			Protocol:    mapping.Protocol,
			TargetIP:    mapping.TargetIP,
			TargetPort:  mapping.TargetPort,
			MappingType: mapping.MappingType,
		})
	}
	return payload
}

func paymentPayloadFromModel(payment *model.Payment) *BuyVPSPaymentPayload {
	if payment == nil {
		return nil
	}
	fee, _ := int64Value(payment.ProviderPayload["fee"])
	totalPayment, _ := int64Value(payment.ProviderPayload["total_payment"])
	qrString, _ := stringValue(payment.ProviderPayload["qr_string"])
	return &BuyVPSPaymentPayload{
		Provider:      derefString(payment.Provider),
		PaymentMethod: payment.Method,
		Amount:        payment.Amount,
		Fee:           fee,
		TotalPayment:  totalPayment,
		QRString:      qrString,
		ExpiredAt:     payment.ExpiredAt,
	}
}

func purchaseStatus(order *model.Order, payment *model.Payment) string {
	if order == nil {
		return "failed"
	}
	if order.Service != nil && order.Service.Instance != nil && order.Service.Status == "active" {
		return "active"
	}
	switch order.Status {
	case "awaiting_payment":
		return "awaiting_payment"
	case "paid":
		return "payment_verified"
	case "processing":
		return "provisioning"
	case "completed":
		if order.Service != nil {
			return "active"
		}
		return "provisioning"
	case "failed":
		return "failed"
	default:
		if payment != nil && payment.Status == "pending" {
			return "awaiting_payment"
		}
		return order.Status
	}
}

func latestPayment(payments []model.Payment) *model.Payment {
	if len(payments) == 0 {
		return nil
	}
	return &payments[len(payments)-1]
}

func extractOneTimePassword(payment *model.Payment) string {
	if payment == nil || payment.ProviderPayload == nil {
		return ""
	}
	delivered, _ := boolValue(payment.ProviderPayload["credentials_delivered"])
	if delivered {
		return ""
	}
	password, _ := stringValue(payment.ProviderPayload["initial_ssh_password"])
	return password
}

func clearOneTimePassword(payload map[string]any) {
	if payload == nil {
		return
	}
	delete(payload, "initial_ssh_password")
	payload["credentials_delivered"] = true
}

func newProvisioningJob(serviceID string, orderID *string, status string, attempts int, actorType string, actorID *string, operationID string, errMsg *string, payload map[string]any) *model.ProvisioningJob {
	now := time.Now().UTC()
	job := &model.ProvisioningJob{
		ID:              uuid.NewString(),
		OrderID:         orderID,
		JobType:         "provision",
		Status:          status,
		RequestedByType: actorType,
		AttemptCount:    attempts,
		ErrorMessage:    errMsg,
		Payload:         payload,
		StartedAt:       &now,
		FinishedAt:      &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if serviceID != "" {
		job.ServiceID = &serviceID
	}
	if strings.TrimSpace(operationID) != "" {
		job.IncusOperationID = &operationID
	}
	if actorType != actorTypeSystem {
		job.RequestedByID = actorID
	}
	return job
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func derefInt(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func intPtr(value int) *int {
	return &value
}

func strPtr(value string) *string {
	return &value
}

func stringValue(value any) (string, bool) {
	str, ok := value.(string)
	return str, ok
}

func boolValue(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	default:
		return false, false
	}
}

func int64Value(value any) (int64, bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	case float64:
		return int64(typed), true
	default:
		return 0, false
	}
}

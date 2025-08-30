package services

import (
	"backend/src/config"
	"backend/src/modules/Order/models"
	"backend/src/repository"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

func CreateOrder(input models.OrderRequestDTO) (*models.OrderResponseDTO, error) {
	if err := validate.Struct(input); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	var CartItems []repository.CartItem
	var OrderItems []repository.OrderItem
	if err := config.DB.Where("user_id = ?", input.UserID).Find(&CartItems).Error; err != nil {
		if len(CartItems) == 0 {
			return nil, fmt.Errorf("no items in cart for user id: %d", input.UserID)
		}
		return nil, fmt.Errorf("failed to retrieve cart items: %w", err)
	} else {

		for _, item := range CartItems {
			var parfume repository.Parfume
			var bottle repository.Bottle
			if err := config.DB.Where("id = ?", item.ParfumeID).First(&parfume).Error; err != nil {
				return nil, fmt.Errorf("failed to retrieve parfume: %w", err)
			}
			if err := config.DB.Where("id = ?", item.BottleID).First(&bottle).Error; err != nil {
				return nil, fmt.Errorf("failed to retrieve bottle: %w", err)
			}
			OrderItems = append(OrderItems, repository.OrderItem{
				ParfumeID: item.ParfumeID,
				BottleID:  item.BottleID,
				Quantity:  item.Quantity,
				Price:     parfume.PriceML + bottle.Price,
			})
		}
		if len(OrderItems) == 0 {
			return nil, fmt.Errorf("no valid items found for order")
		}
	}

	var total float64 = 0
	for _, item := range CartItems {
		var parfume repository.Parfume
		var bottle repository.Bottle

		if err := config.DB.First(&parfume, "id = ?", item.ParfumeID).Error; err != nil {
			return nil, fmt.Errorf("failed to retrieve parfume: %w", err)
		}
		if err := config.DB.First(&bottle, "id = ?", item.BottleID).Error; err != nil {
			return nil, fmt.Errorf("failed to retrieve bottle: %w", err)
		}

		OrderItems = append(OrderItems, repository.OrderItem{
			ParfumeID: item.ParfumeID,
			BottleID:  item.BottleID,
			Quantity:  item.Quantity,
			Price:     parfume.PriceML + bottle.Price,
		})

		total += float64(item.Quantity) * (parfume.PriceML + bottle.Price)
	}

	Order := repository.Order{
		UserID:      input.UserID,
		TotalPrice:  total,
		OrderStatus: repository.OrderStatus{
			ID:   1,
			Status: "pending",
		},
		Items:      OrderItems,
		TotalQuantity: input.TotalQuantity,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := config.DB.Create(&Order).Error; err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	for items := range OrderItems {
		OrderItems[items].OrderID = Order.ID
		if err := config.DB.Create(&OrderItems[items]).Error; err != nil {
			return nil, fmt.Errorf("failed to create order item: %w", err)
		}
	}
	var orderItemDTOs []models.OrderItemDTO
	for _, item := range OrderItems {
		orderItemDTOs = append(orderItemDTOs, models.OrderItemDTO{
			ID:        item.ID,
			ParfumeID: item.ParfumeID,
			BottleID:  item.BottleID,
			Quantity:  item.Quantity,
			Price:     item.Price,
		})
	}
	return &models.OrderResponseDTO{
		ID:         Order.ID,
		UserID:     Order.UserID,
		TotalPrice: Order.TotalPrice,
		OrderStatus: models.OrderStatusDTO{
			ID:   Order.OrderStatusID,
			Name: "pending",
		},
		Items:         orderItemDTOs,
		TotalQuantity: Order.TotalQuantity,
		CreatedAt:     Order.CreatedAt,
		UpdatedAt:     Order.UpdatedAt,
	}, nil
}

func GetOrderByID(orderID uint) (*models.OrderResponseDTO, error) {
	var order repository.Order
	if err := config.DB.Preload("Items").Preload("OrderStatus").First(&order, orderID).Error; err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}

	var orderItemDTOs []models.OrderItemDTO
	for _, item := range order.Items {
		orderItemDTOs = append(orderItemDTOs, models.OrderItemDTO{
			ID:        item.ID,
			ParfumeID: item.ParfumeID,
			BottleID:  item.BottleID,
			Quantity:  item.Quantity,
			Price:     item.Price,
		})
	}

	return &models.OrderResponseDTO{
		ID:         order.ID,
		UserID:     order.UserID,
		TotalPrice: order.TotalPrice,
		OrderStatus: models.OrderStatusDTO{
			ID:   order.OrderStatus.ID,
			Name: order.OrderStatus.Status,
		},
		Items:         orderItemDTOs,
		TotalQuantity: order.TotalQuantity,
		CreatedAt:     order.CreatedAt,
		UpdatedAt:     order.UpdatedAt,
	}, nil
}

func GetOrdersByUserID(userID uint) ([]models.OrderResponseDTO, error) {
	var orders []repository.Order
	if err := config.DB.Preload("Items").Preload("OrderStatus").
		Where("user_id = ?", userID).Find(&orders).Error; err != nil {
		return nil, fmt.Errorf("failed to get orders: %w", err)
	}

	var result []models.OrderResponseDTO
	for _, order := range orders {
		var orderItemDTOs []models.OrderItemDTO
		for _, item := range order.Items {
			orderItemDTOs = append(orderItemDTOs, models.OrderItemDTO{
				ID:        item.ID,
				ParfumeID: item.ParfumeID,
				BottleID:  item.BottleID,
				Quantity:  item.Quantity,
				Price:     item.Price,
			})
		}
		result = append(result, models.OrderResponseDTO{
			ID:         order.ID,
			UserID:     order.UserID,
			TotalPrice: order.TotalPrice,
			OrderStatus: models.OrderStatusDTO{
				ID:   order.OrderStatus.ID,
				Name: order.OrderStatus.Status,
			},
			Items:         orderItemDTOs,
			TotalQuantity: order.TotalQuantity,
			CreatedAt:     order.CreatedAt,
			UpdatedAt:     order.UpdatedAt,
		})
	}

	return result, nil
}

func CancelOrder(orderID uint, userID uint) error {
	var order repository.Order
	if err := config.DB.First(&order, orderID).Error; err != nil {
		return fmt.Errorf("order not found: %w", err)
	}
	if order.UserID != userID {
		return fmt.Errorf("unauthorized: user cannot cancel this order")
	}
	if order.OrderStatusID != 1 { 
		return fmt.Errorf("cannot cancel order with status: %d", order.OrderStatusID)
	}

	order.OrderStatusID = 4 
	order.UpdatedAt = time.Now()

	if err := config.DB.Save(&order).Error; err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}
	return nil
}

func UpdateOrderStatus(orderID uint, status string) error {
	var order repository.Order
	if err := config.DB.First(&order, orderID).Error; err != nil {
		return fmt.Errorf("order not found: %w", err)
	}

	var orderStatus repository.OrderStatus
	if err := config.DB.Where("status = ?", status).First(&orderStatus).Error; err != nil {
		return fmt.Errorf("invalid status: %w", err)
	}

	order.OrderStatusID = orderStatus.ID
	order.UpdatedAt = time.Now()

	if err := config.DB.Save(&order).Error; err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	return nil
}

package pizza

import (
	"errors"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func PizzaWorkflow(ctx workflow.Context, order PizzaOrder) (OrderConfirmation, error) {
	retrypolicy := &temporal.RetryPolicy{
		MaximumInterval: time.Second * 60,
		MaximumAttempts: 100,
		// TODO Part B: Add a `NonRetryableErrorTypes` parameter.
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute * 5,
		RetryPolicy:         retrypolicy,
		// TODO Part D: Add a `HeartbeatTimeout` parameter.
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	logger := workflow.GetLogger(ctx)

	var totalPrice int
	for _, pizza := range order.Items {
		totalPrice += pizza.Price
	}

	var distance Distance
	err := workflow.ExecuteActivity(ctx, GetDistance, order.Address).Get(ctx, &distance)
	if err != nil {
		logger.Error("Unable to get distance", "Error", err)
		return OrderConfirmation{}, err
	}

	if order.IsDelivery && distance.Kilometers > 12 {
		return OrderConfirmation{}, errors.New("Out of Service Area")
	}

	// We use a short Timer duration here to avoid delaying the exercise
	workflow.Sleep(ctx, time.Second*3)

	// TODO Part C: Uncomment this function.
	//err = workflow.ExecuteActivity(ctx, NotifyDeliveryDriver).Get(ctx, nil)
	//if err != nil {
	//	logger.Error("Unable to notify delivery driver.", "Error", err)
	//	return OrderConfirmation{}, err
	//}

	bill := Bill{
		CustomerID:  order.Customer.CustomerID,
		OrderNumber: order.OrderNumber,
		Amount:      totalPrice,
		Description: "Pizza",
	}

	var confirmation OrderConfirmation
	err = workflow.ExecuteActivity(ctx, SendBill, bill).Get(ctx, &confirmation)
	if err != nil {
		logger.Error("Unable to bill customer", "Error", err)
		return OrderConfirmation{}, err
	}

	var chargestatus ChargeStatus
	err = workflow.ExecuteActivity(ctx, ProcessCreditCard, order.Address).Get(ctx, &chargestatus)
	if err != nil {
		var applicationErr *temporal.ApplicationError
		if errors.As(err, &applicationErr) {
			// You could be pushing individual values to a logging system here
			println("Billing timestamp of failed order:", confirmation.BillingTimestamp)
			logger.Error("Unable to charge credit card", "Error", err)
		}

		return OrderConfirmation{}, err
	}

	return confirmation, nil
}

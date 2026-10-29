package main

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controldctl/internal/controld"
)

const billingSensitivityNote = "The output contains financial account information. Scope the token you\n" +
	"hand an autonomous agent accordingly.\n\n" +
	"Read-only: this command never mutates remote state."

func dateString(d controld.Date) string {
	return d.Format(time.DateOnly)
}

func optionalDateString(d *controld.Date) *string {
	if d == nil {
		return nil
	}
	formatted := dateString(*d)
	return &formatted
}

type billingProductPayload struct {
	PK          int    `json:"pk"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Priority    int    `json:"priority"`
	ProxyAccess bool   `json:"proxy_access"`
}

func toBillingProductPayload(p controld.BillingProduct) billingProductPayload {
	return billingProductPayload{
		PK:          p.PK,
		Name:        p.Name,
		Type:        p.Type,
		Priority:    p.Priority,
		ProxyAccess: bool(p.ProxyAccess),
	}
}

// paymentPayload drops the payer fingerprint, the per-currency Stripe price
// tables, and the opaque iv_* fields: none is needed to reconcile spend, and
// the fingerprint identifies the payment instrument. Timestamps are tagged
// non-deterministic; amounts are not, because a recorded payment amount is a
// stable historical fact.
type paymentPayload struct {
	PK             string                `json:"pk"`
	ID             string                `json:"id"`
	TxID           string                `json:"tx_id"`
	SubID          string                `json:"sub_id"`
	Date           string                `json:"date"            ax:"nondeterministic"`
	TxStatus       int                   `json:"tx_status"`
	TxRefunded     bool                  `json:"tx_refunded"`
	User           string                `json:"user"`
	Org            *string               `json:"org"`
	Product        billingProductPayload `json:"product"`
	Amount         float64               `json:"amount"`
	Currency       string                `json:"currency"`
	CurrencyAmount float64               `json:"currency_amount"`
	Balance        float64               `json:"balance"`
	Method         string                `json:"method"`
	Ts             int64                 `json:"ts"              ax:"nondeterministic"`
}

func toPaymentPayload(p controld.Payment) paymentPayload {
	return paymentPayload{
		PK:             p.PK,
		ID:             p.ID,
		TxID:           p.TxID,
		SubID:          p.SubID,
		Date:           dateString(p.Date),
		TxStatus:       p.TxStatus,
		TxRefunded:     bool(p.TxRefunded),
		User:           p.User,
		Org:            p.Org,
		Product:        toBillingProductPayload(p.Product),
		Amount:         p.Amount,
		Currency:       p.Currency,
		CurrencyAmount: p.CurrencyAmount,
		Balance:        p.Balance,
		Method:         p.Method,
		Ts:             p.Ts.Unix(),
	}
}

type billingPaymentsPayload struct {
	Payments []paymentPayload `json:"payments"`
}

type subscriptionPayload struct {
	PK             string                `json:"pk"`
	ID             string                `json:"id"`
	SubID          string                `json:"sub_id"`
	User           string                `json:"user"`
	Org            *string               `json:"org"`
	Product        billingProductPayload `json:"product"`
	Method         string                `json:"method"`
	Currency       string                `json:"currency"`
	Amount         float64               `json:"amount"`
	CurrencyAmount float64               `json:"currency_amount"`
	Started        string                `json:"started"         ax:"nondeterministic"`
	Ended          *string               `json:"ended"           ax:"nondeterministic"`
	CancelReason   *string               `json:"cancel_reason"`
	Price          float64               `json:"price"`
	Status         int                   `json:"status"`
	State          string                `json:"state"`
	NextBill       int64                 `json:"next_bill"       ax:"nondeterministic"`
	NextRebillDate *string               `json:"next_rebill_date" ax:"nondeterministic"`
}

func toSubscriptionPayload(s controld.Subscription) subscriptionPayload {
	return subscriptionPayload{
		PK:             s.PK,
		ID:             s.ID,
		SubID:          s.SubID,
		User:           s.User,
		Org:            s.Org,
		Product:        toBillingProductPayload(s.Product),
		Method:         s.Method,
		Currency:       s.Currency,
		Amount:         s.Amount,
		CurrencyAmount: s.CurrencyAmount,
		Started:        dateString(s.Started),
		Ended:          optionalDateString(s.Ended),
		CancelReason:   s.CancelReason,
		Price:          s.Price,
		Status:         s.Status,
		State:          s.State,
		NextBill:       s.NextBill.Unix(),
		NextRebillDate: optionalDateString(s.NextRebillDate),
	}
}

type billingSubscriptionsPayload struct {
	Subscriptions []subscriptionPayload `json:"subscriptions"`
}

type productSubscriptionPayload struct {
	PK             string   `json:"pk"`
	ID             string   `json:"id"`
	SubID          string   `json:"sub_id"`
	User           string   `json:"user"`
	Org            *string  `json:"org"`
	Product        int      `json:"product"`
	Method         string   `json:"method"`
	Currency       *string  `json:"currency"`
	Amount         float64  `json:"amount"`
	CurrencyAmount *float64 `json:"currency_amount"`
	Started        string   `json:"started"         ax:"nondeterministic"`
	Ended          *string  `json:"ended"           ax:"nondeterministic"`
	CancelReason   *string  `json:"cancel_reason"`
	Price          float64  `json:"price"`
	Status         int      `json:"status"`
	State          string   `json:"state"`
	NextBill       int64    `json:"next_bill"       ax:"nondeterministic"`
}

type pricePointPayload struct {
	PK            int     `json:"pk"`
	ProductID     int     `json:"product_id"`
	Type          string  `json:"type"`
	Duration      int     `json:"duration"`
	Price         float64 `json:"price"`
	AlreadyBilled int     `json:"already_billed"`
	Comment       string  `json:"comment"`
	Status        *int    `json:"status"`
}

type activeProductPayload struct {
	PK           int                        `json:"pk"`
	Name         string                     `json:"name"`
	Type         string                     `json:"type"`
	ProxyAccess  bool                       `json:"proxy_access"`
	Expiry       string                     `json:"expiry"       ax:"nondeterministic"`
	Subscription productSubscriptionPayload `json:"subscription"`
	Price        pricePointPayload          `json:"price"`
}

func toActiveProductPayload(p controld.Product) activeProductPayload {
	sub := p.Subscription
	return activeProductPayload{
		PK:          p.PK,
		Name:        p.Name,
		Type:        p.Type,
		ProxyAccess: bool(p.ProxyAccess),
		Expiry:      dateString(p.Expiry),
		Subscription: productSubscriptionPayload{
			PK:             sub.PK,
			ID:             sub.ID,
			SubID:          sub.SubID,
			User:           sub.User,
			Org:            sub.Org,
			Product:        sub.Product,
			Method:         sub.Method,
			Currency:       sub.Currency,
			Amount:         sub.Amount,
			CurrencyAmount: sub.CurrencyAmount,
			Started:        dateString(sub.Started),
			Ended:          optionalDateString(sub.Ended),
			CancelReason:   sub.CancelReason,
			Price:          sub.Price,
			Status:         sub.Status,
			State:          sub.State,
			NextBill:       sub.NextBill.Unix(),
		},
		Price: pricePointPayload{
			PK:            p.Price.PK,
			ProductID:     p.Price.ProductID,
			Type:          p.Price.Type,
			Duration:      p.Price.Duration,
			Price:         p.Price.Price,
			AlreadyBilled: p.Price.AlreadyBilled,
			Comment:       p.Price.Comment,
			Status:        p.Price.Status,
		},
	}
}

type billingProductsPayload struct {
	Products []activeProductPayload `json:"products"`
}

func newBillingCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "billing",
		Short: "Inspect the account's payments, subscriptions, and active products",
	}
	cmd.AddCommand(newBillingPaymentsCommand(factory))
	cmd.AddCommand(newBillingSubscriptionsCommand(factory))
	cmd.AddCommand(newBillingProductsCommand(factory))
	return cmd
}

func newBillingPaymentsCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "payments",
		Short: "List the account's payment history",
		Long:  "List the account's payment history.\n\n" + billingSensitivityNote,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := factory(cmd)
			if err != nil {
				return err
			}
			payments, err := client.ListPayments(cmd.Context())
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			payload := billingPaymentsPayload{Payments: make([]paymentPayload, 0, len(payments))}
			for _, p := range payments {
				payload.Payments = append(payload.Payments, toPaymentPayload(p))
			}
			return ax.WriteJSON(cmd.OutOrStdout(), newEnvelope(cmd.Context(), payload))
		},
	}
	ax.WithNonDeterministicFields[billingPaymentsPayload](cmd)
	return cmd
}

func newBillingSubscriptionsCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "subscriptions",
		Short: "List the account's subscriptions",
		Long:  "List the account's subscriptions.\n\n" + billingSensitivityNote,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := factory(cmd)
			if err != nil {
				return err
			}
			subscriptions, err := client.ListSubscriptions(cmd.Context())
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			payload := billingSubscriptionsPayload{
				Subscriptions: make([]subscriptionPayload, 0, len(subscriptions)),
			}
			for _, s := range subscriptions {
				payload.Subscriptions = append(payload.Subscriptions, toSubscriptionPayload(s))
			}
			return ax.WriteJSON(cmd.OutOrStdout(), newEnvelope(cmd.Context(), payload))
		},
	}
	ax.WithNonDeterministicFields[billingSubscriptionsPayload](cmd)
	return cmd
}

func newBillingProductsCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "products",
		Short: "List the products active on the account",
		Long: "List the products active on the account, each with its expiry and the\n" +
			"subscription that provides it. This is not a catalog of products the\n" +
			"account could subscribe to.\n\n" + billingSensitivityNote,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := factory(cmd)
			if err != nil {
				return err
			}
			products, err := client.ListActiveProducts(cmd.Context())
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			payload := billingProductsPayload{Products: make([]activeProductPayload, 0, len(products))}
			for _, p := range products {
				payload.Products = append(payload.Products, toActiveProductPayload(p))
			}
			return ax.WriteJSON(cmd.OutOrStdout(), newEnvelope(cmd.Context(), payload))
		},
	}
	ax.WithNonDeterministicFields[billingProductsPayload](cmd)
	return cmd
}

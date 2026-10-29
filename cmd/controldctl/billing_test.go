package main

import (
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"github.com/rshade/ax-go"
)

const paymentsBody = `{"success": true, "body": {"payments": [
	{"PK": "pay1", "id": "id1", "tx_id": "tx1", "sub_id": "sub1", "fingerprint": "fp-secret",
	 "date": "2024-01-15", "tx_status": 1, "tx_refunded": 0, "user": "u1", "org": null,
	 "product": {"PK": 7, "name": "Business", "type": "plan", "priority": 2, "proxy_access": 1},
	 "price_point": {"PK": 3, "product_id": 7, "type": "monthly", "duration": 1, "price": 9.5,
	   "stripe_id": "price_x", "eur_price": 8.8},
	 "amount": 9.5, "currency": "USD", "currency_amount": 9.5, "balance": 0, "method": "card",
	 "ts": 1705312800, "iv_state": null, "iv_meta": null, "brand_new_field": "ignored"}
]}}`

const subscriptionsBody = `{"success": true, "body": {"subscriptions": [
	{"PK": "s1", "id": "id1", "sub_id": "sub1", "user": "u1", "org": null,
	 "product": {"PK": 7, "name": "Business", "type": "plan", "priority": 2, "proxy_access": 1},
	 "method": "card", "currency": "USD", "amount": 9.5, "currency_amount": 9.5,
	 "started": "2024-01-15", "ended": null, "cancel_reason": null, "price": 9.5,
	 "status": 1, "state": "active", "next_bill": 1707991200, "next_rebill_date": "2024-02-15"}
]}}`

const productsBody = `{"success": true, "body": {"products": [
	{"PK": 7, "name": "Business", "type": "plan", "proxy_access": 1, "expiry": "2025-01-15",
	 "subscription": {"PK": "s1", "id": "id1", "sub_id": "sub1", "user": "u1", "org": null,
	   "product": 7, "method": "card", "currency": null, "amount": 9.5, "currency_amount": null,
	   "started": "2024-01-15", "ended": null, "cancel_reason": null, "price": 9.5,
	   "status": 1, "state": "active", "next_bill": 1707991200},
	 "price": {"PK": 3, "already_billed": 0, "comment": "", "duration": 1, "price": 9.5,
	   "product_id": 7, "status": 1, "type": "monthly"}}
]}}`

var snakeCaseKey = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

func assertSnakeCaseKeys(t *testing.T, prefix string, value any) {
	t.Helper()
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if !snakeCaseKey.MatchString(key) {
				t.Errorf("key %s.%s is not lowercase snake_case", prefix, key)
			}
			assertSnakeCaseKeys(t, prefix+"."+key, child)
		}
	case []any:
		for _, child := range v {
			assertSnakeCaseKeys(t, prefix+"[]", child)
		}
	}
}

func cannedHandler(body string, gotPath *string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if gotPath != nil {
			*gotPath = r.Method + " " + r.URL.Path
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}
}

func TestBillingCommandsEmitEnvelope(t *testing.T) {
	cases := []struct {
		sub, wantRequest, listKey, body string
	}{
		{"payments", "GET /billing/payments", "payments", paymentsBody},
		{"subscriptions", "GET /billing/subscriptions", "subscriptions", subscriptionsBody},
		{"products", "GET /billing/products", "products", productsBody},
	}
	for _, tc := range cases {
		t.Run(tc.sub, func(t *testing.T) {
			var gotRequest string
			stdout, stderr, code := executeCommand(t, cannedHandler(tc.body, &gotRequest),
				[]string{"billing", tc.sub, "--format=json"})
			if code != ax.ExitSuccess {
				t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr)
			}
			if gotRequest != tc.wantRequest {
				t.Fatalf("request = %q, want %q", gotRequest, tc.wantRequest)
			}

			var envelope struct {
				Data map[string][]map[string]any `json:"data"`
			}
			if err := json.Unmarshal(stdout, &envelope); err != nil {
				t.Fatalf("unmarshal stdout %q: %v", stdout, err)
			}
			if len(envelope.Data[tc.listKey]) != 1 {
				t.Fatalf("data.%s = %v, want one record", tc.listKey, envelope.Data[tc.listKey])
			}

			var generic any
			if err := json.Unmarshal(stdout, &generic); err != nil {
				t.Fatalf("unmarshal stdout: %v", err)
			}
			assertSnakeCaseKeys(t, "$", generic)
		})
	}
}

func TestBillingPaymentsDropsFingerprintAndKeepsAmounts(t *testing.T) {
	stdout, stderr, code := executeCommand(t, cannedHandler(paymentsBody, nil),
		[]string{"billing", "payments", "--format=json"})
	if code != ax.ExitSuccess {
		t.Fatalf("exit code = %d; stderr=%s", code, stderr)
	}
	for _, leaked := range []string{"fp-secret", "fingerprint", "price_x", "brand_new_field"} {
		if strings.Contains(string(stdout), leaked) {
			t.Errorf("stdout contains %q, want it omitted", leaked)
		}
	}
	for _, want := range []string{`"amount":9.5`, `"date":"2024-01-15"`, `"ts":1705312800`, `"pk":"pay1"`} {
		if !strings.Contains(string(stdout), want) {
			t.Errorf("stdout = %s, want it to contain %s", stdout, want)
		}
	}
}

func TestBillingEmptyResultsAreEmptyLists(t *testing.T) {
	cases := map[string]string{
		"payments":      `{"success": true, "body": {"payments": []}}`,
		"subscriptions": `{"success": true, "body": {"subscriptions": []}}`,
		"products":      `{"success": true, "body": {"products": []}}`,
	}
	for sub, body := range cases {
		t.Run(sub, func(t *testing.T) {
			stdout, stderr, code := executeCommand(t, cannedHandler(body, nil),
				[]string{"billing", sub, "--format=json"})
			if code != ax.ExitSuccess {
				t.Fatalf("exit code = %d; stderr=%s", code, stderr)
			}
			if !strings.Contains(string(stdout), `"data":{"`+sub+`":[]}`) {
				t.Fatalf("stdout = %s, want an empty list rather than null", stdout)
			}
		})
	}
}

func TestBillingUnparseableBodyFailsWithoutSuccessEnvelope(t *testing.T) {
	stdout, _, code := executeCommand(t, cannedHandler("<html>not json</html>", nil),
		[]string{"billing", "payments", "--format=json"})
	if code == ax.ExitSuccess {
		t.Fatalf("exit code = %d, want a failure for an unparseable body", code)
	}
	if strings.Contains(string(stdout), `"data"`) {
		t.Fatalf("stdout = %s, want no partial success envelope", stdout)
	}
}

func TestBillingHelpDeclaresSensitivityAndOffersNoMutationFlags(t *testing.T) {
	for _, sub := range []string{"payments", "subscriptions", "products"} {
		t.Run(sub, func(t *testing.T) {
			var cmdLong string
			var flags []string
			billing := newBillingCommand(noopFactory)
			for _, c := range billing.Commands() {
				if c.Name() == sub {
					cmdLong = c.Long
					c.LocalFlags().VisitAll(func(f *pflag.Flag) { flags = append(flags, f.Name) })
				}
			}
			if !strings.Contains(cmdLong, "financial account information") {
				t.Errorf("billing %s help = %q, want a financial-data sensitivity note", sub, cmdLong)
			}
			for _, banned := range []string{"yes", "dry-run"} {
				if slices.Contains(flags, banned) {
					t.Errorf("billing %s declares --%s, want a read-only command without it", sub, banned)
				}
			}
		})
	}
}

func TestBillingSchemaTagsTimestampsButNotAmounts(t *testing.T) {
	cases := []struct {
		sub      string
		tagged   []string
		untagged []string
	}{
		{"payments", []string{"data.payments.ts", "data.payments.date"}, []string{"data.payments.amount", "data.payments.balance"}},
		{"subscriptions", []string{"data.subscriptions.started", "data.subscriptions.next_bill"}, []string{"data.subscriptions.amount", "data.subscriptions.price"}},
		{"products", []string{"data.products.expiry"}, []string{"data.products.price.price", "data.products.subscription.amount"}},
	}
	for _, tc := range cases {
		t.Run(tc.sub, func(t *testing.T) {
			fields := schemaFor(t, "billing", tc.sub).NonDeterministicFields
			for _, want := range tc.tagged {
				if !slices.Contains(fields, want) {
					t.Errorf("non_deterministic_fields = %v, want it to contain %s", fields, want)
				}
			}
			for _, banned := range tc.untagged {
				if slices.Contains(fields, banned) {
					t.Errorf("non_deterministic_fields = %v, want %s absent: amounts are stable historical facts", fields, banned)
				}
			}
		})
	}
}

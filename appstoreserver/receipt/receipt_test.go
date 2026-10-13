package receipt

import (
	"os"
	"testing"
)

func TestAppleReceiptVectors(t *testing.T) {
	for _, test := range []struct {
		path, want string
		legacy     bool
	}{
		{"xcode/xcode-app-receipt-empty", "", false},
		{"xcode/xcode-app-receipt-with-transaction", "0", false},
		{"mock_signed_data/legacyTransaction", "33993399", true},
	} {
		t.Run(test.path, func(t *testing.T) {
			data, err := os.ReadFile("../testdata/apple/" + test.path)
			if err != nil {
				t.Fatal(err)
			}
			var got string
			if test.legacy {
				got, err = (ReceiptUtility{}).ExtractTransactionIDFromTransactionReceipt(string(data))
			} else {
				got, err = (ReceiptUtility{}).ExtractTransactionIDFromAppReceipt(string(data))
			}
			if err != nil || got != test.want {
				t.Fatalf("got %q %v, want %q", got, err, test.want)
			}
		})
	}
}

func FuzzAppReceipt(f *testing.F) {
	f.Add("invalid")
	f.Add("MIA=")
	f.Fuzz(func(t *testing.T, data string) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		_, _ = (ReceiptUtility{}).ExtractTransactionIDFromAppReceipt(data)
	})
}

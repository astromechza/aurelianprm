package digest

// newSMTPSender returns a senderFunc backed by real SMTP (implemented in Task 3).
// This stub satisfies the compiler until go-mail is wired in.
func newSMTPSender(_ Config) senderFunc {
	return func(_, _, _, _ string) error {
		panic("newSMTPSender not yet implemented — run Task 3")
	}
}

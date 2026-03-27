package semantic

import "fmt"

func buildSystemPrompt(subject string, expectation string) string {
	return fmt.Sprintf(
		"You are evaluating whether a response satisfies an expectation for %s. "+
			"Return a score from 1 to 10 where 10 means the response fully satisfies the expectation and anything below 5 means the response is far from acceptable. "+
			"Use the provided expectation as the source of truth and explain the score briefly. "+
			"The expectation is: %s",
		subject,
		expectation,
	)
}

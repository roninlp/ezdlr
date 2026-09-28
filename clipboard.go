package main

import (
	"regexp"
	"strings"
)

type ClipboardURL struct {
	URL    string `json:"url"`
	Reason string `json:"reason,omitempty"`
}

type ClipboardReview struct {
	Accepted   []ClipboardURL `json:"accepted"`
	Duplicates []ClipboardURL `json:"duplicates"`
	Rejected   []ClipboardURL `json:"rejected"`
}

func (r ClipboardReview) AcceptedCount() int { return len(r.Accepted) }

func (r ClipboardReview) DuplicateCount() int { return len(r.Duplicates) }

func (r ClipboardReview) RejectedCount() int { return len(r.Rejected) }

type ClipboardResult struct {
	URL    string        `json:"url"`
	Status string        `json:"status"`
	Reason string        `json:"reason,omitempty"`
	Item   *DownloadItem `json:"item,omitempty"`
}

type ClipboardBatchResult struct {
	Results []ClipboardResult `json:"results"`
}

func (r ClipboardBatchResult) AcceptedCount() int {
	return countClipboardResults(r.Results, "accepted")
}

func (r ClipboardBatchResult) DuplicateCount() int {
	return countClipboardResults(r.Results, "duplicate")
}

func (r ClipboardBatchResult) RejectedCount() int {
	return countClipboardResults(r.Results, "rejected")
}

func (r ClipboardBatchResult) EnqueueFailureCount() int {
	return countClipboardResults(r.Results, "enqueue-failure")
}

func countClipboardResults(results []ClipboardResult, status string) int {
	count := 0
	for _, result := range results {
		if result.Status == status {
			count++
		}
	}
	return count
}

var clipboardURLPattern = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://[^\s]+`)

func reviewClipboardText(text string, items []DownloadItem) ClipboardReview {
	review := ClipboardReview{
		Accepted:   []ClipboardURL{},
		Duplicates: []ClipboardURL{},
		Rejected:   []ClipboardURL{},
	}
	existing := make(map[string]struct{}, len(items))
	for _, item := range items {
		existing[item.URL] = struct{}{}
	}
	seen := make(map[string]struct{})
	for _, rawURL := range clipboardURLPattern.FindAllString(text, -1) {
		candidate := strings.TrimSpace(rawURL)
		cleanURL, err := validateURL(candidate)
		if err != nil {
			review.Rejected = append(review.Rejected, ClipboardURL{URL: candidate, Reason: err.Error()})
			continue
		}
		if _, duplicate := seen[cleanURL]; duplicate {
			review.Duplicates = append(review.Duplicates, ClipboardURL{URL: cleanURL, Reason: "URL appears more than once in this batch"})
			continue
		}
		seen[cleanURL] = struct{}{}
		if _, duplicate := existing[cleanURL]; duplicate {
			review.Duplicates = append(review.Duplicates, ClipboardURL{URL: cleanURL, Reason: "URL is already represented in the queue"})
			continue
		}
		review.Accepted = append(review.Accepted, ClipboardURL{URL: cleanURL})
	}
	return review
}

func (s *DownloadService) ReviewClipboard(text string) ClipboardReview {
	return reviewClipboardText(text, s.loop.itemsSnapshot())
}

// ConfirmClipboard enqueues the reviewed links in queueID. The queue choice
// belongs to the caller because it decides whether the batch starts: the
// default clipboard queue is stopped, so a confirmed batch waits until the
// user starts it.
func (s *DownloadService) ConfirmClipboard(review ClipboardReview, queueID string) (ClipboardBatchResult, error) {
	if queueID == "" {
		queueID = ClipboardQueueID
	}
	if err := s.loop.queueExists(queueID); err != nil {
		return ClipboardBatchResult{}, err
	}
	return s.loop.confirmClipboard(review, queueID), nil
}

func (l *downloadEngineLoop) confirmClipboard(review ClipboardReview, queueID string) ClipboardBatchResult {
	l.mu.Lock()
	defer l.mu.Unlock()

	result := ClipboardBatchResult{Results: []ClipboardResult{}}
	for _, entry := range review.Rejected {
		result.Results = append(result.Results, ClipboardResult{URL: entry.URL, Status: "rejected", Reason: entry.Reason})
	}
	for _, entry := range review.Duplicates {
		result.Results = append(result.Results, ClipboardResult{URL: entry.URL, Status: "duplicate", Reason: entry.Reason})
	}

	// Recheck accepted URLs because the review may have been held while another
	// action added one of them.
	for _, entry := range review.Accepted {
		cleanURL, err := validateURL(entry.URL)
		if err != nil {
			result.Results = append(result.Results, ClipboardResult{URL: entry.URL, Status: "rejected", Reason: err.Error()})
			continue
		}
		if l.hasURLLocked(cleanURL) {
			result.Results = append(result.Results, ClipboardResult{URL: cleanURL, Status: "duplicate", Reason: "URL is already represented in the queue"})
			continue
		}
		item, err := l.enqueueLocked(cleanURL, queueID)
		if err != nil {
			result.Results = append(result.Results, ClipboardResult{URL: cleanURL, Status: "enqueue-failure", Reason: err.Error()})
			continue
		}
		itemCopy := item
		result.Results = append(result.Results, ClipboardResult{URL: cleanURL, Status: "accepted", Item: &itemCopy})
	}
	return result
}

// Reviews are value objects, so cancelling one only requires the caller to
// discard it. This method keeps cancellation explicit at the app boundary.
func (s *DownloadService) CancelClipboardReview() {}

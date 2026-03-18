package domain

import "unicode/utf8"

const (
	MaxContentBytes = 100 * 1024 // 100KB
	MinContentBytes = 1
)

// ValidateStoreRequest validates all fields of a StoreRequest
func ValidateStoreRequest(req *StoreRequest) error {
	errs := &ValidationErrors{}

	if req.Content == "" {
		errs.Add("content", "content is required")
	} else if !utf8.ValidString(req.Content) {
		errs.Add("content", "content must be valid UTF-8")
	} else if len(req.Content) < MinContentBytes {
		errs.Add("content", "content must be at least 1 byte")
	} else if len(req.Content) > MaxContentBytes {
		errs.Add("content", "content must not exceed 100KB")
	}

	if req.Type != "" && !req.Type.IsValid() {
		errs.Add("type", "invalid memory type: "+string(req.Type))
	}

	for i, tag := range req.Tags {
		if tag == "" {
			errs.Add("tags", "tag at index "+itoa(i)+" must not be empty")
		}
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// ValidateUpdateRequest validates all fields of an UpdateRequest
func ValidateUpdateRequest(req *UpdateRequest) error {
	errs := &ValidationErrors{}

	if req.ID == "" {
		errs.Add("id", "id is required")
	}

	if req.Content != nil {
		if *req.Content == "" {
			errs.Add("content", "content must not be empty")
		} else if !utf8.ValidString(*req.Content) {
			errs.Add("content", "content must be valid UTF-8")
		} else if len(*req.Content) > MaxContentBytes {
			errs.Add("content", "content must not exceed 100KB")
		}
	}

	if req.Type != nil && !req.Type.IsValid() {
		errs.Add("type", "invalid memory type")
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for i > 0 {
		buf = append([]byte{byte('0' + i%10)}, buf...)
		i /= 10
	}
	return string(buf)
}

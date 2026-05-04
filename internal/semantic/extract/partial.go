package extract

// PartialExtract builds an ExtractedFile representing a partial or
// unsupported extraction outcome. Used by:
//
//   - The Phase 59 P03 scheduler when a file's classified language returns
//     "other" (D-05 unsupported_language path — file row only, no
//     symbols/refs/imports written).
//   - Per-language providers when parse_error / query_error / timeout /
//     file_too_large / binary_or_generated / permission_denied / extractor_bug
//     fires (D-05 first-class-with-errors path — partial facts may still be
//     written by the caller; this helper produces the FileFact envelope).
//
// `reason` MUST be one of the closed-enum PartialReason values (CONTEXT.md
// D-05). Unsupported-language and file-too-large and binary-or-generated
// outcomes set ExtractionStatus = ExtractionStatusUnsupported (the file
// was seen but cannot produce facts). All other reasons set
// ExtractionStatus = ExtractionStatusPartial (partial facts may have been
// written).
func PartialExtract(file SourceFile, reason PartialReason, err error) *ExtractedFile {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	status := ExtractionStatusPartial
	switch reason {
	case PartialReasonUnsupportedLanguage,
		PartialReasonFileTooLarge,
		PartialReasonBinaryOrGenerated:
		status = ExtractionStatusUnsupported
	}
	return &ExtractedFile{
		File: FileFact{
			Path:              file.Path,
			Language:          file.Language,
			ExtractionStatus:  status,
			ExtractionPartial: true,
			PartialReason:     reason,
			ErrorMessage:      msg,
		},
		Partial:       true,
		PartialReason: string(reason),
	}
}

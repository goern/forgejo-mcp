<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

## 1. Upload source

- [x] 1.1 Add shared source validation and file opening in `pkg/upload`
- [x] 1.2 Add `file_path` and exclusive-source validation to all three tools
- [x] 1.3 Infer path basenames while preserving explicit filename overrides

## 2. Multipart transport

- [x] 2.1 Stream multipart request bodies through `io.Pipe`
- [x] 2.2 Route release attachment creation through the shared HTTP helper

## 3. Verification and documentation

- [x] 3.1 Cover base64, file paths, validation, and streaming with tests
- [x] 3.2 Update README, demos, and historical attachment design notes
- [x] 3.3 Run the full test suite and build

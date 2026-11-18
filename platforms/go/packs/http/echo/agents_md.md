---
target: agents_md
order: 310
rules: [http-echo.usage]
---
## HTTP: Echo

- Decode with `c.Bind`; validation runs only through the validator registered on `e.Validator` and `c.Validate`, so check that one is set.
- Return `echo.NewHTTPError(code, message)` or a domain error, and map domain errors in the one `e.HTTPErrorHandler`.
- Pass `c.Request().Context()` to the service, not the `echo.Context`.

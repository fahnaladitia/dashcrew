---
target: role/flutter-code-reviewer
order: 320
rules: [network.dioexception-import-allowed]
---
### Dio usage

- Importing `package:dio` only to handle `DioException` is not a violation. Search for `Dio(`, `BaseOptions(` and `interceptors` outside `{{pack.network.client_dir}}`, not for the import.

Bruno fixtures
==============

The .bru files in this directory are copied unmodified from Bruno 2.0.0
(https://github.com/usebruno/bruno), which is MIT licensed. See
../../../../THIRD_PARTY_NOTICES.md and ../../../../docs/BRU_FORMAT.md §10.

They are used only as parser conformance input. No Bruno source code is copied
into apigo; the Go implementation is clean-room.

| File                             | Origin                                                                                        |
| -------------------------------- | --------------------------------------------------------------------------------------------- |
| request.bru                      | packages/bruno-lang/v2/tests/fixtures/request.bru                                             |
| annotations.bru                  | packages/bruno-lang/v2/tests/fixtures/annotations.bru                                         |
| collection.bru                   | packages/bruno-lang/v2/tests/fixtures/collection.bru                                          |
| oauth2-examples.bru              | packages/bruno-lang/v2/tests/examples/fixtures/bru/oauth2-examples.bru                        |
| examples-complex.bru             | packages/bruno-lang/v2/tests/examples/fixtures/bru/examples-complex.bru                       |
| multiple-examples-variations.bru | packages/bruno-lang/v2/tests/examples/fixtures/bru/multiple-examples-variations.bru           |
| grpc-request.bru                 | tests/ssl/client-certs/tests/global-client-certs-enabled/fixtures/collection/grpc-request.bru |
| wss-request.bru                  | tests/ssl/client-certs/tests/global-client-certs-enabled/fixtures/collection/wss-request.bru  |
| oauth2-env.bru                   | packages/bruno-tests/keycloak-password-credentials/environments/oauth2.bru                    |

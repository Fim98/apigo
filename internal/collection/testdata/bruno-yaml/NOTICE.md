Bruno OpenCollection fixtures
=============================

The YAML collections in this directory are copied unmodified from Bruno 2.0.0
(https://github.com/usebruno/bruno), which is MIT licensed. See
../../../../THIRD_PARTY_NOTICES.md and ../../../../docs/OPENCOLLECTION.md §8.

They are used only as reader conformance input. No Bruno source code is copied
into apigo; the Go implementation is clean-room.

| Directory       | Origin                                                                       |
| --------------- | ---------------------------------------------------------------------------- |
| `edgegrid/`     | tests/auth/akamai-edgegrid/fixtures/collections/yml                          |
| `descriptions/` | tests/collection/description-yml/fixtures/collection                         |
| `folders/`      | tests/scripting/nodevm-developer-mode/fixtures/collections/async-callbacks    |
| `graphql/`      | tests/graphql/query-builder/fixtures/collection                              |

`folders/` deliberately contains both `bruno.json` and `opencollection.yml`, so a
test can pin the "OpenCollection wins, warn about the shadowed format" rule.

# Wire Format

## Steps File (`file`)

| size | name  | description        |
|------|-------|--------------------|
| 4 B  | magic | "hino" in ASCII    |
| ?    | steps | n (n >= 0) `step`s |

## Single Step (`step`)

| size      | name    | description                  |
|-----------|---------|------------------------------|
| 8 B       | bufSize | size of buffer (below)       |
| bufSize B | buf     | buffer (encoded in protobuf) |

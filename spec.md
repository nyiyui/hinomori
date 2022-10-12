#Spec

## Order

```
0755 /
    0755 bin
    0755 boot
    ...
    0755 home
```
- `/`
- `/bin`
- `/boot`
- `/home`
- `/bin/1password2john`

## Wire format

| bytes   | desc             |
|---------|------------------|
| 4       | file mode (UNIX) |
| 2       | owner            |
| 2       | group            |
| 8       | file size [B]    |
| 2       | filename size [B] | 
| 1-255   | filename         |
| 1       | NUL character (padding for safety |

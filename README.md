[![version](https://img.shields.io/github/tag/z0rr0/unigma.svg)](https://github.com/z0rr0/unigma/releases/latest) [![license](https://img.shields.io/github/license/z0rr0/unigma.svg)](https://github.com/z0rr0/unigma/blob/master/LICENSE)

# Unigma

Secure file sharing service.

Process:

- upload a file (+settings: required password + TTL + number of sharing)
- the file is encrypted using AES-256 with a key based on user's password, metadata is stored in local SQLite database and private info is also encrypted
- get unique link
- share the link (recipient should know used password)


## Build

Dependencies:

```
github.com/mattn/go-sqlite3
golang.org/x/crypto/pbkdf2
golang.org/x/crypto/sha3
```

Check and build

```bash
make install
```

Prepare empty database `db.sqlite`:

```bash
cat schema.sql | sqlite3 db.sqlite
```

For docker container

```bash
make docker
```

## Development

### Run

```bash
make start

make restart

make stop
```

## License

This source code is governed by a MIT license that can be found in the [LICENSE](https://github.com/z0rr0/unigma/blob/master/LICENSE) file.

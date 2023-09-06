# BoltStore - Session store using BoltDB

## Overview

BoltStore is a session store using [Bolt](https://go.etcd.io/bbolt) which is a pure Go key/value store. You can store session data in Bolt by using this store. This store implements the [gorilla/sessions](https://github.com/gorilla/sessions) package's [Store](http://godoc.org/github.com/gorilla/sessions#Store) interface.
 
Based on [Redistore](https://github.com/boj/redistore) codebase.
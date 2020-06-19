# Durations

Durations are lengths of time that are specified as part of a pluign configuration using a number or string. 

If a number is specified, it will be interpreted as a number of seconds.

If a string is specified, it will be interpreted according to Golang's [`time.ParseDuration`](https://golang.org/src/time/format.go?s=40541:40587#L1369) documentation. 

## Examples

### Various ways to specify a duration of 1 minute

```yaml
- id: my_plugin
  type: some_plugin
  duration: 1m
```

```yaml
- id: my_plugin
  type: some_plugin
  duration: 60s
```

```yaml
- id: my_plugin
  type: some_plugin
  duration: 60
```

```yaml
- id: my_plugin
  type: some_plugin
  duration: 60.0
```
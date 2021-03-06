## `google_cloud_output` operator

The `google_cloud_output` operator will send entries to Google Cloud Logging.

### Configuration Fields

| Field              | Default               | Description                                                                                            |
| ---                | ---                   | ---                                                                                                    |
| `id`               | `google_cloud_output` | A unique identifier for the operator                                                                   |
| `credentials`      |                       | The JSON-formatted credentials for the logs writer service account                                     |
| `credentials_file` |                       | A path to a file containing the JSON-formatted credentials                                             |
| `project_id`       |                       | The Google Cloud project ID the logs should be sent to. Defaults to project_id found in credentials    |
| `log_name_field`   |                       | A [field](/docs/types/field.md) for the log name on the entry. Log name defaults to `default` if unset |
| `labels_field`     |                       | A [field](/docs/types/field.md) for the labels object on the log entry                                 |
| `severity_field`   |                       | A [field](/docs/types/field.md) for the severity on the log entry                                      |
| `trace_field`      |                       | A [field](/docs/types/field.md) for the trace on the log entry                                         |
| `span_id_field`    |                       | A [field](/docs/types/field.md) for the span_id on the log entry                                       |

If both `credentials` and `credentials_file` are left empty, the agent will attempt to find
[Application Default Credentials](https://cloud.google.com/docs/authentication/production) from the environment.

### Example Configurations

#### Simple configuration

Configuration:
```yaml
- type: google_cloud_output
  project_id: sample_project
  credentials_file: /tmp/credentials.json
```

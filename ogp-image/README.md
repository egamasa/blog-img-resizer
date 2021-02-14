# Make OGP Image

## Command-line arguments

- `-e`
  - Element images path to insert (1 or 2)
  - Split by `,` when 2 images
- `-t`
  - Insert text
- `-p`
  - Font size of insert text
- `-o`
  - Output file name


## Configurations

`config.json`


## Example

```
$ go run main.go \
    -e gcp/cloud-functions.png,lang/ruby.png \
    -t "Cloud Functions × Ruby" \
    -p 48 \
    -o gcf-ruby
```

![gcf-ruby.jpg](./doc/gcf-ruby.jpg)

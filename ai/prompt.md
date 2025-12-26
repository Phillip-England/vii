---
$at("*")
$skill("rpp")
$skill("ergo")
$skill("modular")
---

Okay I need a way to associate static files with a specific endpoint. But I want to do this in way where I can choose between dyanamic or static embedding.

For example, if I want my static files to be embedded in my output, we need to gather them all up and then pass them from some sort of map when files are called upon.

But if they are dynamic, we can just read from disk when a file is requests.

I should be able to define a static DIR and pass it to my app:

```go
embedDir, err := someFunctionOrType
// deal with err idk if the path is right might need method not sure
app.ServeEmbeddedFiles("/static"  embedDir)

// for dyanamic dfile from disk
app.ServeLocalFiles("/static", "/path/to/dir/on/disk")
```

that way i have the best of both worlds

This also leads to the idea of having assets that are embedded and available during a request that are not intedned for static handling. LEts imagine we wanted to embed an html template or a dir full of them or something like that.

We could say:
```go
type EmbeddedDirKey string
var EmbeddedDirKey = "someKey"
embedDir, err := someFunctionOrType
// deal with err
app.EmbedDir(embedDir, EmbeddedDirKey)

// then in a route
d, ok := rt.GetEmbeddedDir(EmbeddedDirKey)
fileStr, ok := d.Get("some", "path", "to", "file.md")
```
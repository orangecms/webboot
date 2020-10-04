## Usage

The webboot program would do the following:
 - Present a menu with the existing cached distro options
 - If the user wants a distro that is not cached, they can download an ISO 
 - After the user decides on an ISO, boot it.

## Testing

Our UI uses a package called Termui. Termui will parse the standard input into
keyboard events and insert them into a channel, from which Termui then get its
input. To implement an unattended test, I manually build a series of keyboard
events that represent my intented input for testing, and insert them into a
channel. Then I replace the original input channel with my channel in the test.
So `go test` could run a test of the UI automatically.

See `TestDownloadOption` for an example:
 - create a channel by `make(chan ui.Event)`.
 - use go `pressKey(uiEvents, input)` to translate the intented test input to
   keyboard events and push them to the `uiEvents` channel.
 - use the `uiEvents` channel by calling `downloadOption.exec(uiEvents)`. (Main
   function will always call `ui.PollEvents()` to get the standard input channel) 
 - all functions involving UI input will provide an argument to indicate the
   input channel.

## Caching

If you want to set up a cached directory on the USB stick, the file structure
should be as follows. Note that `Images` *must* be the name of the cache
directory by convention.

```
  +-- USB root
  |  +-- Images
  |  |  +-- subdirectories or iso files
  ...
```

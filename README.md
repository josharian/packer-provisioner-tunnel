packer-provisioner-tunnel is a [custom provisioner](https://www.packer.io/docs/extend/provisioner.html) for [packer](https://www.packer.io/).
It executes an external tool such as [serverspec](http://serverspec.org/) and provides that tool an SSH tunnel into the image.
The SSH credentials and port are passed to the tool using environment variables.
Be sure to see the limitations section below.


### Building and installation

This is vanilla Go. Build with `go build` or `go install`. Don't change the binary name. (It should be packer-provisioner-tunnel.) Put the binary in your path. Packer will pick it up automatically.


### Use

Here is an example Packer spec:

```json
{
  "builders": [...],
  "provisioners": [{
      "type": "tunnel",
      "exec": "rake",
      "args": ["-v"]
    }]
}
```

This will execute `rake -v` with tunnel SSH information as environment variables.


### SSH details

Tunnel supports only username/password authentication, because there is no reason to support anything deeper. The environment variables are self-explanatory:

```
PACKER_TUNNEL_USERNAME
PACKER_TUNNEL_PASSWORD
PACKER_TUNNEL_PORT
```

Because tunnel generates a new server key each time, you must turn off strict host checking.


### serverspec

Here is a sample `spec_helper.rb` snippet to enable serverspec to play nicely with tunnel:

```ruby
# auth
options[:auth_methods] = ["password"]
options[:user] = ENV['PACKER_TUNNEL_USERNAME']
options[:password] = ENV['PACKER_TUNNEL_PASSWORD']

# server
options[:host_name] = "127.0.0.1"
options[:port] = ENV['PACKER_TUNNEL_PORT']

# disable strict host checking
options[:paranoid] = false

set :ssh_options, options

# if sudo is enabled, it must be non-interactive
set :disable_sudo, true
```


### Limitations

The tunnel does not (yet) support Packer templates. This is not a fundamental limitation; they're just not implemented yet.

The tunnel supports only a limited subset of SSH. In particular, it does not provide a shell or a pty. That means that you cannot use "ssh" as your external tool, and you cannot use interactive commands, most notably sudo. As far as I can tell, this is a fundamental limitation. Packer does not actually expose an SSH connection to its provisioner plugins, among other reasons because it does not always have one. The tunnel thus emulates an SSH backend that knows only how to execute commands.


### TODOs

* There's not much by way of comments and docs. This is because I don't know yet whether this is throw-away code.

* I'm genuinely unsure how to write reasonable tests for this that aren't significant more complicated and fragile than the code being tested.

* Support Packer templates.

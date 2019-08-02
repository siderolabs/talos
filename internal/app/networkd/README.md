# networkd

Networkd handles the addressing and interface configuration in Talos.

The general workflow is:

- Discover all network interfaces ( `networkd.Discover()` )
- Create an abstract representation of the network interface configuration  ( `nic.NetworkInterface` )
- Merge userdata configuration options on top of the `nic.NetworkInterface` representation
- Configure the network interfaces based on the abstract representation ( `networkd.Configure(...)` )
- - Bring interface up
- - Begin address configuration method ( `address.DHCP`, `address.Static` )
- - Create rtnetlink message to set address based on config method
- - Create rtnetlink message to set any routes defined by the address method

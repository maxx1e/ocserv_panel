# occtl output examples

## command: `occtl help`

```text
Available Commands
 disconnect user [NAME] Disconnect the specified user
 disconnect id [ID]     Disconnect the specified ID
     unban ip [IP]      Unban the specified IP
           reload       Reloads the server configuration
      show status       Prints the status and statistics of the server
       show users       Prints the connected users
     show ip bans       Prints the banned IP addresses
 show ip ban points     Prints all the known IP addresses which have points
     show iroutes       Prints the routes provided by users of the server
 show sessions all      Prints all the session IDs
 show sessions valid    Prints all the valid for reconnection sessions
 show session [SID]     Prints information on the specified session
    show user [NAME]    Prints information on the specified user
      show id [ID]      Prints information on the specified ID
      show events       Provides information about connecting users
         stop now       Terminates the server
            reset       Resets the screen and terminal
         help or ?      Prints this help
             exit       Exits this application
```

## command: `occtl -j -n show status`

```json
 {
    "Status":  "online",
    "Server PID":  3108823,
    "Sec-mod PID":  3108828,
    "Sec-mod instance count":  1,
    "Up since":  "2026-05-04 07:29",
    "_Up since":  " 5days",
    "raw_up_since":  1777872597,
    "uptime":  463303,
    "Active sessions":  2,
    "Total sessions":  30,
    "Total authentication failures":  9,
    "IPs in ban list":  0,
    "Last stats reset":  "2026-05-04 07:29",
    "_Last stats reset":  " 5days",
    "raw_last_stats_reset":  1777872597,
    "Sessions handled":  30,
    "Timed out sessions":  0,
    "Timed out (idle) sessions":  0,
    "Closed due to error sessions":  16,
    "Authentication failures":  9,
    "Average auth time":  "    2s",
    "raw_avg_auth_time":  2,
    "Max auth time":  "   10s",
    "raw_max_auth_time":  10,
    "Average session time":  " 7h:40m",
    "raw_avg_session_time":  27600,
    "Max session time":  " 5days",
    "raw_max_session_time":  449520,
    "Min MTU":  1284,
    "Max MTU":  1292,
    "RX":  "122.3 MB",
    "raw_rx":  122275000,
    "TX":  "3.6 GB",
    "raw_tx":  3619653000
  }
```

## command: `occtl -j -n show iroutes`

```json
    {
    "ID":  3549163,
    "Username":  "maxxie",
    "vhost":  "default",
    "Device":  "vpns0",
    "IP":  "10.10.10.208",
    "iRoutes":  [],
    "IP":  "10.10.10.208"
    "ID":  3488123,
    "Username":  "maxxie",
    "vhost":  "default",
    "Device":  "vpns1",
    "IP":  "10.10.10.228",
    "iRoutes":  [],
    "IP":  "10.10.10.228"
  }
```

## command: `occtl -j -n show sessions all`

```json
[
  {
    "Session":  "YwDQr5",
    "Full session":  "YwDQr54wGdgKCAju+hgsYQcJLc0=",
    "Created":  "2026-05-09 12:24",
    "State":  "authenticated",
    "Username":  "maxxie",
    "Groupname":  "(none)",
    "vhost":  "default",
    "User-Agent":  "Open AnyConnect VPN Agent 9.12-3",
    "Remote IP":  "5.166.153.114",
    "Location":  "unknown",
    "session_is_open":  1,
    "tls_auth_ok":  0,
    "in_use":  1,
  },

  {
    "Session":  "fySBxd",
    "Full session":  "fySBxdqjHqDbKTT4g/IoG4H0VYY=",
    "Created":  "2026-05-08 15:07",
    "State":  "authenticated",
    "Username":  "maxxie",
    "Groupname":  "(none)",
    "vhost":  "default",
    "User-Agent":  "AnyConnect Android 5.1.15.295",
    "Remote IP":  "94.204.188.26",
    "Location":  "unknown",
    "session_is_open":  1,
    "tls_auth_ok":  0,
    "in_use":  1,
  }
]
```

## command: `occtl -j -n show session YwDQr5`

```json
[
  {
    "Session":  "YwDQr5",
    "Full session":  "YwDQr54wGdgKCAju+hgsYQcJLc0=",
    "Created":  "2026-05-09 12:24",
    "State":  "authenticated",
    "Username":  "maxxie",
    "Groupname":  "(none)",
    "vhost":  "default",
    "User-Agent":  "Open AnyConnect VPN Agent 9.12-3",
    "Remote IP":  "5.166.153.114",
    "Location":  "unknown",
    "session_is_open":  1,
    "tls_auth_ok":  0,
    "in_use":  1,
  },
]
```

## command: `occtl -j -n show users`

```json
[
  {
    "ID":  3549163,
    "Username":  "maxxie",
    "Groupname":  "(none)",
    "State":  "connected",
    "vhost":  "default",
    "Device":  "vpns0",
    "MTU":  "1292",
    "Remote IP":  "5.166.153.114",
    "Location":  "unknown",
    "Local Device IP":  "188.68.41.144",
    "IPv4":  "10.10.10.208",
    "P-t-P IPv4":  "10.10.10.1",
    "User-Agent":  "Open AnyConnect VPN Agent 9.12-3",
    "RX":  "304",
    "TX":  "304",
    "_RX":  "304 bytes",
    "_TX":  "304 bytes",
    "Average RX":  "0 bytes/s",
    "Average TX":  "0 bytes/s",
    "DPD":  "90",
    "KeepAlive":  "32400",
    "Hostname":  "LemonGrass",
    "Connected at":  "2026-05-09 12:23",
    "_Connected at":  " 3h:52m",
    "raw_connected_at":  1778322239,
    "Full session":  "YwDQr54wGdgKCAju+hgsYQcJLc0=",
    "Session":  "YwDQr5",
    "TLS ciphersuite":  "(TLS1.3)-(ECDHE-SECP256R1)-(ECDSA-SECP256R1-SHA256)-(CHACHA20-P",
    "DTLS cipher":  "(DTLS1.2)-(PSK)-(CHACHA20-POLY1305)",
    "DNS":      ["1.1.1.1", "9.9.9.9"],
    "NBNS":     [],
    "Split-DNS-Domains":        [],
    "Routes":   [],
    "Routes":  "defaultroute",
    "No-routes":        [],
    "iRoutes":  [],
    "Restricted to routes":  "False",
    "Restricted to ports":      []
  },

  {
    "ID":  3488123,
    "Username":  "maxxie",
    "Groupname":  "(none)",
    "State":  "connected",
    "vhost":  "default",
    "Device":  "vpns1",
    "MTU":  "1284",
    "Remote IP":  "94.204.188.26",
    "Location":  "unknown",
    "Local Device IP":  "188.68.41.144",
    "IPv4":  "10.10.10.228",
    "P-t-P IPv4":  "10.10.10.1",
    "User-Agent":  "AnyConnect Android 5.1.15.295",
    "RX":  "31683792",
    "TX":  "994919220",
    "_RX":  "31.7 MB",
    "_TX":  "994.9 MB",
    "Average RX":  "350 bytes/s",
    "Average TX":  "11.0 kB/s",
    "DPD":  "90",
    "KeepAlive":  "32400",
    "Hostname":  "S10",
    "Connected at":  "2026-05-08 15:07",
    "_Connected at":  "25h:08m",
    "raw_connected_at":  1778245667,
    "Full session":  "fySBxdqjHqDbKTT4g/IoG4H0VYY=",
    "Session":  "fySBxd",
    "TLS ciphersuite":  "(TLS1.3)-(ECDHE-SECP256R1)-(ECDSA-SECP256R1-SHA256)-(AES-128-GC",
    "DTLS cipher":  "(DTLS1.2)-(ECDHE-RSA)-(AES-128-GCM)",
    "DNS":      ["1.1.1.1", "9.9.9.9"],
    "NBNS":     [],
    "Split-DNS-Domains":        [],
    "Routes":   [],
    "Routes":  "defaultroute",
    "No-routes":        [],
    "iRoutes":  [],
    "Restricted to routes":  "False",
    "Restricted to ports":      []
  }
]
 ```

## command: `occtl -n -j show user maxxie`

```json
[
  {
    "ID":  3549163,
    "Username":  "maxxie",
    "Groupname":  "(none)",
    "State":  "connected",
    "vhost":  "default",
    "Device":  "vpns0",
    "MTU":  "1292",
    "Remote IP":  "5.166.153.114",
    "Location":  "unknown",
    "Local Device IP":  "188.68.41.144",
    "IPv4":  "10.10.10.208",
    "P-t-P IPv4":  "10.10.10.1",
    "User-Agent":  "Open AnyConnect VPN Agent 9.12-3",
    "RX":  "304",
    "TX":  "304",
    "_RX":  "304 bytes",
    "_TX":  "304 bytes",
    "Average RX":  "0 bytes/s",
    "Average TX":  "0 bytes/s",
    "DPD":  "90",
    "KeepAlive":  "32400",
    "Hostname":  "LemonGrass",
    "Connected at":  "2026-05-09 12:23",
    "_Connected at":  " 3h:52m",
    "raw_connected_at":  1778322239,
    "Full session":  "YwDQr54wGdgKCAju+hgsYQcJLc0=",
    "Session":  "YwDQr5",
    "TLS ciphersuite":  "(TLS1.3)-(ECDHE-SECP256R1)-(ECDSA-SECP256R1-SHA256)-(CHACHA20-P",
    "DTLS cipher":  "(DTLS1.2)-(PSK)-(CHACHA20-POLY1305)",
    "DNS":      ["1.1.1.1", "9.9.9.9"],
    "NBNS":     [],
    "Split-DNS-Domains":        [],
    "Routes":   [],
    "Routes":  "defaultroute",
    "No-routes":        [],
    "iRoutes":  [],
    "Restricted to routes":  "False",
    "Restricted to ports":      []
  },

  {
    "ID":  3488123,
    "Username":  "maxxie",
    "Groupname":  "(none)",
    "State":  "connected",
    "vhost":  "default",
    "Device":  "vpns1",
    "MTU":  "1284",
    "Remote IP":  "94.204.188.26",
    "Location":  "unknown",
    "Local Device IP":  "188.68.41.144",
    "IPv4":  "10.10.10.228",
    "P-t-P IPv4":  "10.10.10.1",
    "User-Agent":  "AnyConnect Android 5.1.15.295",
    "RX":  "31700355",
    "TX":  "995051116",
    "_RX":  "31.7 MB",
    "_TX":  "995.1 MB",
    "Average RX":  "350 bytes/s",
    "Average TX":  "11.0 kB/s",
    "DPD":  "90",
    "KeepAlive":  "32400",
    "Hostname":  "S10",
    "Connected at":  "2026-05-08 15:07",
    "_Connected at":  "25h:09m",
    "raw_connected_at":  1778245667,
    "Full session":  "fySBxdqjHqDbKTT4g/IoG4H0VYY=",
    "Session":  "fySBxd",
    "TLS ciphersuite":  "(TLS1.3)-(ECDHE-SECP256R1)-(ECDSA-SECP256R1-SHA256)-(AES-128-GC",
    "DTLS cipher":  "(DTLS1.2)-(ECDHE-RSA)-(AES-128-GCM)",
    "DNS":      ["1.1.1.1", "9.9.9.9"],
    "NBNS":     [],
    "Split-DNS-Domains":        [],
    "Routes":   [],
    "Routes":  "defaultroute",
    "No-routes":        [],
    "iRoutes":  [],
    "Restricted to routes":  "False",
    "Restricted to ports":      []
  }
]
```

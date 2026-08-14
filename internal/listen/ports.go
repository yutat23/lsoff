package listen

import "strings"

type portKey struct {
	proto Proto
	port  uint16
}

type portMeta struct {
	name    string
	aliases []string
}

var ports = map[portKey]portMeta{}

func init() {
	// Yes: IANA assignment and still widely used.
	// Unofficial: not IANA, but common on developer machines.
	// Assigned-but-unused and Historic (echo, chargen, …) are omitted.
	tcp(20, "ftp-data")
	tcp(21, "ftp")
	both(22, "ssh")
	tcp(23, "telnet")
	tcp(25, "smtp")
	both(53, "dns", "domain")
	udp(67, "dhcp", "bootps")
	udp(68, "dhcp-client", "bootpc")
	udp(69, "tftp")
	tcp(80, "http")
	both(88, "kerberos")
	tcp(110, "pop3")
	both(111, "rpcbind", "sunrpc")
	tcp(119, "nntp")
	udp(123, "ntp")
	both(135, "msrpc")
	udp(137, "netbios-ns")
	udp(138, "netbios-dgm")
	tcp(139, "netbios-ssn")
	tcp(143, "imap")
	udp(161, "snmp")
	udp(162, "snmptrap")
	tcp(179, "bgp")
	tcp(389, "ldap")
	tcp(443, "https")
	tcp(445, "smb", "microsoft-ds")
	tcp(465, "smtps")
	udp(500, "isakmp")
	udp(514, "syslog")
	tcp(515, "lpd")
	tcp(548, "afp")
	tcp(587, "submission", "smtp")
	tcp(631, "ipp", "cups")
	tcp(636, "ldaps")
	tcp(993, "imaps")
	tcp(995, "pop3s")
	udp(5353, "mdns")
	tcp(5432, "postgres", "postgresql")
	tcp(3306, "mysql")
	tcp(1433, "mssql")
	tcp(1521, "oracle")
	tcp(2049, "nfs")
	tcp(2379, "etcd")
	tcp(2380, "etcd-peer")
	tcp(3389, "rdp")
	tcp(5672, "amqp")
	tcp(5900, "vnc")
	tcp(6379, "redis")
	tcp(8080, "http", "http-alt")
	tcp(8008, "http")
	tcp(8443, "https")
	tcp(9200, "elasticsearch")
	tcp(11211, "memcached")
	tcp(27017, "mongodb", "mongo")
	tcp(27018, "mongodb")
	tcp(4222, "nats")
	tcp(2375, "docker")
	tcp(2376, "docker-tls")
	tcp(6443, "kubernetes", "k8s")
	tcp(10250, "kubelet")
	tcp(5173, "vite")
	tcp(9229, "node-inspect")
	tcp(7687, "neo4j")
	tcp(8161, "activemq")
	tcp(9418, "git")
	tcp(853, "dot", "dns-over-tls")
	udp(853, "doq")
	tcp(8888, "jupyter")
	tcp(4200, "angular")
	tcp(5174, "vite")
	tcp(24678, "vite")
	// Ambiguous display names: search aliases only.
	alias(TCP, 3000, "rails", "grafana", "node")
	alias(TCP, 3001, "react", "node")
	alias(TCP, 5000, "flask", "airplay")
	alias(TCP, 8000, "django", "deno")
	alias(TCP, 8081, "http")
	alias(TCP, 9090, "prometheus", "cockpit")
	alias(TCP, 9091, "prometheus")
	alias(TCP, 16686, "jaeger")
	alias(TCP, 4317, "otlp")
	alias(TCP, 4318, "otlp")
}

func tcp(port uint16, name string, extra ...string) {
	put(TCP, port, name, extra...)
}

func udp(port uint16, name string, extra ...string) {
	put(UDP, port, name, extra...)
}

func both(port uint16, name string, extra ...string) {
	put(TCP, port, name, extra...)
	put(UDP, port, name, extra...)
}

func put(p Proto, port uint16, name string, extra ...string) {
	k := portKey{p, port}
	m := ports[k]
	if m.name == "" {
		m.name = name
	}
	m.aliases = appendUniq(m.aliases, name)
	m.aliases = appendUniq(m.aliases, extra...)
	ports[k] = m
}

func alias(p Proto, port uint16, names ...string) {
	k := portKey{p, port}
	m := ports[k]
	m.aliases = appendUniq(m.aliases, names...)
	ports[k] = m
}

func appendUniq(dst []string, in ...string) []string {
	seen := make(map[string]struct{}, len(dst)+len(in))
	for _, s := range dst {
		seen[strings.ToLower(s)] = struct{}{}
	}
	for _, s := range in {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		dst = append(dst, s)
	}
	return dst
}

// ServiceName is a short display label, or empty if unknown or ambiguous.
func ServiceName(p Proto, port uint16) string {
	return ports[portKey{p, port}].name
}

// SearchTerms are lowercase aliases used by FilterQuery (includes ServiceName).
func SearchTerms(p Proto, port uint16) []string {
	return ports[portKey{p, port}].aliases
}

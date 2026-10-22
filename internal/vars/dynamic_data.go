package vars

// The word lists behind the dynamic variables. They are deliberately small and
// plausible rather than a port of faker's locale data: apigo promises the same
// set of names and the same value shape, not faker's exact distribution. A
// collection that needs faker's vocabulary can still run its own JS.

var (
	firstNames = []string{
		"Ada", "Alan", "Grace", "Linus", "Margaret", "Dennis", "Barbara", "Ken",
		"Radia", "Guido", "Katherine", "Bjarne", "Frances", "Tim", "Anita", "Rob",
		"Hedy", "Donald", "Shafi", "Vint", "Sophie", "Yann", "Mira", "Omar",
	}

	lastNames = []string{
		"Lovelace", "Turing", "Hopper", "Torvalds", "Hamilton", "Ritchie", "Liskov",
		"Thompson", "Perlman", "Rossum", "Johnson", "Stroustrup", "Allen", "Berners-Lee",
		"Borg", "Pike", "Lamarr", "Knuth", "Goldwasser", "Cerf", "Wilson", "LeCun",
		"Nakamura", "Haddad",
	}

	namePrefixes = []string{"Mr.", "Mrs.", "Ms.", "Dr.", "Prof."}
	nameSuffixes = []string{"Jr.", "Sr.", "II", "III", "IV", "PhD", "MD"}

	jobAreas = []string{
		"Engineering", "Marketing", "Operations", "Finance", "Design", "Sales",
		"Research", "Support", "Legal", "Product", "Security", "Data",
	}

	jobDescriptors = []string{
		"Lead", "Senior", "Principal", "Global", "Regional", "Chief", "Internal",
		"Dynamic", "Forward", "Corporate", "Direct", "Future",
	}

	jobTypes = []string{
		"Engineer", "Manager", "Analyst", "Coordinator", "Specialist", "Consultant",
		"Architect", "Administrator", "Director", "Designer", "Scientist", "Officer",
	}

	cities = []string{
		"Shanghai", "Lisbon", "Toronto", "Nairobi", "Osaka", "Berlin", "Austin",
		"Melbourne", "Reykjavík", "Medellín", "Tallinn", "Busan", "Porto", "Kyiv",
		"Chennai", "Zurich", "Dakar", "Bogotá", "Helsinki", "Auckland", "Prague",
		"Cape Town", "Seattle", "Valencia",
	}

	streetNames = []string{
		"Oak", "Maple", "Cedar", "Birch", "Willow", "Harbor", "Station", "Market",
		"Church", "Park", "Lake", "Hill", "River", "Bridge", "Garden", "Mill",
	}

	streetSuffixes = []string{"Street", "Avenue", "Road", "Lane", "Drive", "Way", "Court", "Place"}

	countryCodes = []string{
		"CN", "US", "GB", "DE", "FR", "JP", "BR", "IN", "CA", "AU", "ES", "IT",
		"NL", "SE", "NO", "PL", "PT", "KR", "ZA", "MX", "AR", "TR", "EG", "NZ",
	}

	countries = []string{
		"China", "United States", "United Kingdom", "Germany", "France", "Japan",
		"Brazil", "India", "Canada", "Australia", "Spain", "Italy", "Netherlands",
		"Sweden", "Norway", "Poland", "Portugal", "South Korea", "South Africa",
		"Mexico", "Argentina", "Türkiye", "Egypt", "New Zealand",
	}

	domainWords = []string{
		"acme", "northwind", "globex", "initech", "umbrella", "hooli", "stark",
		"wayne", "cyberdyne", "soylent", "wonka", "tyrell", "aperture", "vandelay",
	}

	domainSuffixes = []string{"com", "net", "org", "io", "dev", "co", "app", "cloud"}

	companySuffixes = []string{
		"Inc", "LLC", "Ltd", "Group", "Holdings", "Labs", "Systems", "Partners",
		"Industries", "Solutions", "Works", "Technologies",
	}

	catchPhraseAdjectives = []string{
		"Adaptive", "Advanced", "Automated", "Balanced", "Configurable", "Distributed",
		"Enhanced", "Exclusive", "Expanded", "Innovative", "Integrated", "Optimized",
		"Organic", "Proactive", "Robust", "Seamless", "Streamlined", "Versatile",
	}

	catchPhraseDescriptors = []string{
		"24-hour", "24/7", "client-driven", "coherent", "composite", "dynamic",
		"explicit", "holistic", "modular", "multi-layered", "next-generation",
		"object-oriented", "radical", "real-time", "scalable", "systematic",
	}

	catchPhraseNouns = []string{
		"ability", "access", "adapter", "algorithm", "architecture", "capability",
		"challenge", "circuit", "complexity", "concept", "conglomeration", "framework",
		"function", "Graphic Interface", "hardware", "help-desk", "hierarchy",
		"implementation", "interface", "knowledge base", "matrices", "methodology",
		"middleware", "migration", "model", "monitoring", "paradigm", "pipeline",
		"productivity", "projection", "protocol", "service-desk", "solution",
		"strategy", "structure", "success", "superstructure", "support", "synergy",
		"system engine", "task-force", "throughput", "time-frame", "toolset",
		"utilisation", "workforce",
	}

	buzzAdjectives = []string{
		"24/7", "B2B", "backend", "best-of-breed", "bleeding-edge", "clicks-and-mortar",
		"collaborative", "cross-platform", "distributed", "dot-com", "efficient",
		"end-to-end", "enterprise", "frictionless", "front-end", "global", "granular",
		"holistic", "innovative", "integrated", "killer", "leading-edge", "magnetic",
		"mission-critical", "next-generation", "one-to-one", "open-source", "out-of-the-box",
		"plug-and-play", "real-time", "revolutionary", "scalable", "seamless", "synergistic",
		"turn-key", "ubiquitous", "user-centric", "value-added", "vertical", "viral",
		"virtual", "visionary", "web-enabled", "wireless", "world-class",
	}

	buzzVerbs = []string{
		"aggregate", "architect", "benchmark", "brand", "cultivate", "deliver",
		"deploy", "disintermediate", "drive", "e-enable", "embrace", "empower",
		"enable", "engage", "enhance", "envisioneer", "evolve", "expedite",
		"exploit", "extend", "facilitate", "generate", "grow", "harness",
		"implement", "incentivize", "incubate", "innovate", "integrate",
		"iterate", "leverage", "matrix", "maximize", "mesh", "monetize",
		"morph", "optimize", "orchestrate", "productize", "recontextualize",
		"redefine", "reengineer", "reinvent", "repurpose", "revolutionize",
		"scale", "seize", "strategize", "streamline", "syndicate", "synergize",
		"synthesize", "target", "transform", "unleash", "utilize", "visualize",
		"whiteboard",
	}

	buzzNouns = []string{
		"action-items", "applications", "architectures", "bandwidth", "channels",
		"communities", "content", "convergence", "deliverables", "e-business",
		"e-commerce", "e-markets", "e-services", "e-tailers", "experiences",
		"eyeballs", "functionalities", "infomediaries", "infrastructures",
		"initiatives", "interfaces", "markets", "methodologies", "metrics",
		"mindshare", "models", "networks", "niches", "paradigms", "partnerships",
		"platforms", "portals", "relationships", "ROI", "schemas", "services",
		"supply-chains", "synergies", "systems", "technologies", "users",
		"vortals", "web services", "web-readiness",
	}

	databaseColumns = []string{
		"id", "name", "email", "created_at", "updated_at", "status", "owner_id",
		"title", "slug", "payload", "amount", "currency", "deleted_at",
	}

	databaseTypes = []string{
		"int", "bigint", "varchar", "text", "boolean", "timestamp", "numeric",
		"uuid", "jsonb", "date", "real", "smallint",
	}

	databaseCollations = []string{
		"utf8_general_ci", "utf8mb4_unicode_ci", "en_US.UTF-8", "C", "POSIX",
		"en_US.utf8", "de_DE.UTF-8", "binary",
	}

	databaseEngines = []string{
		"InnoDB", "MyISAM", "MEMORY", "CSV", "ARCHIVE", "BLACKHOLE", "ndbinfo", "ndbcluster",
	}

	weekdays = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

	months = []string{
		"January", "February", "March", "April", "May", "June", "July", "August",
		"September", "October", "November", "December",
	}

	currencyCodes = []string{"USD", "EUR", "CNY", "JPY", "GBP", "CHF", "AUD", "CAD", "INR", "BRL", "SEK", "KRW"}

	currencyNames = []string{
		"US Dollar", "Euro", "Chinese Yuan", "Japanese Yen", "Pound Sterling",
		"Swiss Franc", "Australian Dollar", "Canadian Dollar", "Indian Rupee",
		"Brazilian Real", "Swedish Krona", "South Korean Won",
	}

	currencySymbols = []string{"$", "€", "¥", "£", "CHF", "A$", "C$", "₹", "R$", "kr", "₩"}

	transactionTypes = []string{"deposit", "withdrawal", "payment", "invoice", "transfer", "refund"}

	accountNames = []string{
		"Checking Account", "Savings Account", "Business Account", "Money Market",
		"Credit Line", "Investment Account",
	}

	productAdjectives = []string{
		"Sleek", "Rustic", "Ergonomic", "Handcrafted", "Refined", "Intelligent",
		"Gorgeous", "Practical", "Fantastic", "Licensed", "Generic", "Tasty",
		"Unbranded", "Awesome", "Incredible", "Small", "Durable", "Oriental",
	}

	productMaterials = []string{
		"Steel", "Wooden", "Concrete", "Plastic", "Cotton", "Granite", "Rubber",
		"Metal", "Soft", "Fresh", "Frozen", "Marble", "Bamboo", "Leather",
	}

	products = []string{
		"Chair", "Car", "Computer", "Keyboard", "Mouse", "Bike", "Ball", "Gloves",
		"Pants", "Shirt", "Table", "Shoes", "Hat", "Towels", "Soap", "Tuna",
		"Chicken", "Fish", "Cheese", "Bacon", "Pizza", "Salad", "Sausages",
	}

	departments = []string{
		"Books", "Movies", "Music", "Games", "Electronics", "Computers", "Home",
		"Garden", "Tools", "Grocery", "Health", "Beauty", "Toys", "Kids", "Sports",
		"Outdoors", "Automotive", "Industrial",
	}

	hackerAbbreviations = []string{
		"TCP", "HTTP", "SDD", "RAM", "GB", "CSS", "SSL", "AGP", "SQL", "FTP",
		"PCI", "AI", "ADP", "RSS", "XML", "EXE", "COM", "HDD", "THX", "SMTP",
		"SMS", "USB", "PNG", "XSS", "JSON", "YAML", "JWT", "OAuth",
	}

	hackerAdjectives = []string{
		"auxiliary", "primary", "back-end", "digital", "open-source", "virtual",
		"cross-platform", "redundant", "online", "haptic", "multi-byte", "bluetooth",
		"wireless", "1080p", "solid state", "mobile", "optical", "neural",
	}

	hackerNouns = []string{
		"driver", "protocol", "bandwidth", "panel", "microchip", "program",
		"port", "card", "array", "interface", "system", "sensor", "firewall",
		"hard drive", "pixel", "alarm", "feed", "monitor", "application", "transmitter",
		"bus", "circuit", "capacitor", "matrix", "router", "buffer", "cache", "socket",
	}

	hackerVerbs = []string{
		"back up", "bypass", "hack", "override", "compress", "copy", "navigate",
		"index", "connect", "generate", "quantify", "calculate", "synthesize",
		"input", "transmit", "program", "reboot", "parse",
	}

	hackerIngverbs = []string{
		"backing up", "bypassing", "hacking", "overriding", "compressing",
		"copying", "navigating", "indexing", "connecting", "generating",
		"quantifying", "calculating", "synthesizing", "transmitting", "programming",
		"parsing",
	}

	loremWords = []string{
		"alias", "consequatur", "aut", "perferendis", "sit", "voluptatem",
		"accusantium", "doloremque", "aperiam", "eaque", "ipsa", "quae", "ab",
		"illo", "inventore", "veritatis", "et", "quasi", "architecto", "beatae",
		"vitae", "dicta", "sunt", "explicabo", "aspernatur", "odit", "fugit",
		"sed", "quia", "consequuntur", "magni", "dolores", "eos", "qui", "ratione",
		"sequi", "nesciunt", "neque", "dolorem", "ipsum", "dolor", "amet",
		"consectetur", "adipisci", "velit", "non", "numquam", "eius", "modi",
		"tempora", "incidunt", "ut", "labore", "dolore", "magnam", "aliquam",
		"quaerat", "enim", "ad", "minima", "veniam", "quis", "nostrum",
		"exercitationem", "ullam", "corporis", "nemo", "ipsam", "voluptas",
		"suscipit", "laboriosam", "nisi", "aliquid", "ex", "ea", "commodi",
		"autem", "vel", "eum", "iure", "reprehenderit", "in", "voluptate",
		"esse", "quam", "nihil", "molestiae", "iusto", "odio", "dignissimos",
		"ducimus", "blanditiis", "praesentium", "laudantium", "totam", "rem",
		"voluptatum", "deleniti", "atque", "corrupti", "quos", "quas", "molestias",
		"excepturi", "sint", "occaecati", "cupiditate", "provident", "perspiciatis",
		"unde", "omnis", "iste", "natus", "error", "similique", "culpa", "officia",
		"deserunt", "mollitia", "animi", "id", "est", "laborum", "dolorum", "fuga",
		"harum", "quidem", "rerum", "facilis", "expedita", "distinctio", "nam",
		"libero", "tempore", "cum", "soluta", "nobis", "eligendi", "optio",
		"cumque", "impedit", "quo", "porro", "quisquam", "minus", "quod",
		"maxime", "placeat", "facere", "possimus", "assumenda", "repellendus",
		"temporibus", "quibusdam", "officiis", "debitis", "saepe", "eveniet",
		"voluptates", "repudiandae", "recusandae", "itaque", "earum", "hic",
		"tenetur", "a", "sapiente", "delectus", "reiciendis", "voluptatibus",
		"maiores", "doloribus", "asperiores", "repellat",
	}

	fileExts = []string{"pdf", "docx", "xlsx", "pptx", "txt", "csv", "json", "png", "jpg", "md", "zip", "mp4"}

	commonFileExts = []string{"pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "txt", "csv", "png", "jpg", "gif", "mp3", "mp4", "zip"}

	mimeTypes = []string{
		"application/json", "application/pdf", "application/zip", "application/xml",
		"application/octet-stream", "text/plain", "text/csv", "text/html",
		"image/png", "image/jpeg", "image/svg+xml", "audio/mpeg", "video/mp4",
	}

	fileTypes = []string{"application", "audio", "image", "message", "model", "multipart", "text", "video"}

	userAgentTemplates = []string{
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%d.0.%d.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%d.0.%d.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64; rv:%d.0) Gecko/20100101 Firefox/%d.0",
		"Mozilla/5.0 (iPhone; CPU iPhone OS %d_%d like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/%d.0 Mobile/15E148 Safari/604.1",
	}

	colorNames = []string{
		"red", "orange", "yellow", "green", "blue", "indigo", "violet", "purple",
		"pink", "brown", "black", "white", "gray", "cyan", "magenta", "teal",
		"navy", "olive", "maroon", "lime", "aqua", "silver", "gold", "beige",
	}

	bitcoinAlphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	bech32Alphabet  = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"
	nanoIDAlphabet  = "useandom-26T198340PX75pxJACKVERYMINDBUSHWOLF_GQZbfghjklqvwyzrict"
)

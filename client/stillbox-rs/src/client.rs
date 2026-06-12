use reqwest::{Client, Url, header};
use url::ParseError;

pub const VERSION: &str = env!("CARGO_PKG_VERSION");
pub const NAME: &str = env!("CARGO_PKG_NAME");
pub const USER_AGENT: &str = constcat::concat!(NAME, "/", VERSION);

pub struct RESTClient {
    base_url: Url,
    pub client: Client,
}

impl RESTClient {
    pub fn is_https(&self) -> bool {
        self.base_url.scheme() == "https"
    }

    pub fn websocket_url(&self) -> Result<Url, url::ParseError> {
        let scheme = match self.is_https() {
            true => "wss",
            false => "ws",
        };

        let mut result = self.base_url.clone();
        result.set_scheme(scheme).unwrap();
        result.join("/api/ws")
    }
}

mod auth;

pub fn new_client(base_url: &str) -> Result<RESTClient, Box<dyn std::error::Error>> {
    let mut agent_hdrs = header::HeaderMap::new();
    agent_hdrs.insert("User-Agent", header::HeaderValue::from_static(USER_AGENT));

    let cb = reqwest::ClientBuilder::new().cookie_store(true).default_headers(agent_hdrs);
    let client = cb.build()?;

    match Url::parse(base_url) {
        Ok(url) => {
            Ok(RESTClient {
                base_url: url,
                client,
            })
        },
        Err(err) => Err(Box::new(err)),
    }
}
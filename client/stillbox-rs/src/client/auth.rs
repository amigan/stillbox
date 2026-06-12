use core::fmt;
use std::str::FromStr;

use url::ParseError;
use super::RESTClient;
use serde::{Deserialize};

#[derive(Deserialize)]
pub struct JWT {
    jwt: String,
}

#[derive(Debug)]
pub enum AuthError {
    HttpError(reqwest::Error),
    LoginError(String),
    ParseError(ParseError),
}

impl std::error::Error for AuthError {}
impl fmt::Display for AuthError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self {
            AuthError::HttpError(e) => write!(f, "HTTP Error: {}", e.to_string()),
            AuthError::ParseError(e) => write!(f, "Parse Error: {}", e.to_string()),
            AuthError::LoginError(e) => write!(f, "Login Error: {}", e.to_string()),
        }
    }
}

impl RESTClient {
    pub async fn login(&self, username: &str, password: &str) -> Result<JWT, AuthError> {
        let params = [("username", username), ("password", password)];
        let url = match self.base_url.join("/api/login") {
            Ok(u) => u,
            Err(err) => return Err(AuthError::ParseError(err)),
        };
        let req = self.client.post(url).form(&params);
        let res = match req.send().await {
            Ok(resp) => resp,
            Err(err) => return Err(AuthError::HttpError(err)),
        };
        if res.status() != 200 {
            let rtxt = res.text().await.unwrap_or(String::from_str("no body").unwrap());
            return Err(AuthError::LoginError(rtxt));
        }
        match res.json::<JWT>().await {
            Ok(jwt) => Ok(jwt),
            Err(err) => return Err(AuthError::HttpError(err)),
        }
    }
}

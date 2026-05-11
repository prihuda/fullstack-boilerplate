export interface User {
  id: string;
  email: string;
  name: string;
  created_at: string;
}

export interface LoginRequest {
  email: string;
  password: string;
}

/** Response body from /auth/login — tokens are delivered via HttpOnly cookies,
 *  so this type is only used for the TypeScript generic on `post<TokenResponse>`.
 *  The frontend never reads these token values client-side. */
export interface TokenResponse {
  access_token: string;
  token_type: string;
  refresh_token: string;
  expires_in: number;
  expires_at: string;
}

export interface ApiError {
  code: string;
  message: string;
  details?: Record<string, string>;
}

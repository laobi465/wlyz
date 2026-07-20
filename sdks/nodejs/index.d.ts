// Type definitions for keyauth-node v0.3.6

export interface LoginOptions {
  deviceName?: string;
  deviceType?: string; // windows/macos/linux/android/ios/web
}

export interface ClientOptions {
  apiBase: string;
  appKey: string;
  signSecret: string;
  timeout?: number;
  appSecret?: string;
}

export interface CardInfo {
  type: string;
  status: string;
  expires_at?: number;
  remaining_seconds: number;
  bound_devices: number;
  max_devices: number;
  used_count: number;
  max_uses: number;
}

export interface DeviceInfo {
  id: number;
  hwid: string;
  name: string;
  bound_at: number;
}

export interface LoginResult {
  token?: string;
  expires_at: number;
  card: CardInfo;
  device: DeviceInfo;
  heartbeat_interval: number;
  heartbeat_timeout: number;
}

export interface VerifyResult {
  card: CardInfo;
  device: { id: number; hwid: string };
  last_heartbeat_at: number;
  heartbeat_interval: number;
  heartbeat_timeout: number;
}

export interface HeartbeatResult {
  next_heartbeat: number;
  heartbeat_timeout: number;
  server_time: number;
}

export interface BindResult {
  device_id: number;
  bound_at: number;
  bound_count: number;
  max_devices: number;
}

export interface UnbindResult {
  unbound: boolean;
  deducted_seconds: number;
  message: string;
}

export interface GetVarResult {
  var_key: string;
  var_value: string;
  var_type: string;
  updated_at: number;
}

export interface NoticeItem {
  id: number;
  title: string;
  content: string;
  is_pinned: boolean;
  created_at: number;
}

export interface NoticeResult {
  notices: NoticeItem[];
}

export interface VersionResult {
  has_update: boolean;
  force_update: boolean;
  latest_version: string;
  current_version: string;
  download_url: string;
  backup_url: string;
  update_description: string;
  min_version: string;
  released_at: number;
}

export interface LogoutResult {
  logged_out: boolean;
}

export class KeyAuthError extends Error {
  code: number;
  message: string;
  httpStatus: number;
  constructor(code: number, message: string, httpStatus?: number);
}

export class KeyAuthClient {
  constructor(opts: ClientOptions);
  login(cardKey: string, hwid: string, opts?: LoginOptions): Promise<LoginResult>;
  verify(cardKey: string, hwid: string): Promise<VerifyResult>;
  heartbeat(cardKey: string, hwid: string): Promise<HeartbeatResult>;
  bind(cardKey: string, hwid: string, opts?: LoginOptions): Promise<BindResult>;
  unbind(cardKey: string, hwid: string): Promise<UnbindResult>;
  getVar(cardKey: string, varKey: string): Promise<GetVarResult>;
  notice(): Promise<NoticeResult>;
  version(currentVersion?: string, platform?: string): Promise<VersionResult>;
  logout(cardKey: string, hwid: string): Promise<LogoutResult>;
}

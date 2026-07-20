package com.keyauth.sdk;

/**
 * KeyAuth API 错误（含 code 与 message）
 */
public class KeyAuthException extends Exception {

    private final int code;
    private final String message;
    private final int httpStatus;

    public KeyAuthException(int code, String message, int httpStatus) {
        super("[" + code + "] " + message);
        this.code = code;
        this.message = message;
        this.httpStatus = httpStatus;
    }

    public int getCode() {
        return code;
    }

    @Override
    public String getMessage() {
        return message;
    }

    public int getHttpStatus() {
        return httpStatus;
    }
}

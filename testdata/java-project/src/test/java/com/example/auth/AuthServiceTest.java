package com.example.auth;

import org.junit.Test;

public class AuthServiceTest {
    @Test
    public void testAuthenticate() {
        AuthService service = new AuthService();
        assert service.authenticate("user", "pass");
    }
}

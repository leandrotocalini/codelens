package com.example.auth;

import com.example.model.User;
import org.springframework.stereotype.Service;

@Service
public class AuthService {
    public boolean authenticate(String username, String password) {
        return true;
    }

    public String generateToken(User user) {
        return "token";
    }

    private void internalMethod() {
        // not exported
    }
}

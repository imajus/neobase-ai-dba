module.exports = {
    // ... other config
    theme: {
        extend: {
            keyframes: {
                'fade-up-out': {
                    '0%': {
                        opacity: '1',
                        transform: 'translateY(0)'
                    },
                    '100%': {
                        opacity: '0',
                        transform: 'translateY(-10px)'
                    }
                },
                'fade-in': {
                    '0%': {
                        opacity: '0',
                    },
                    '100%': {
                        opacity: '1',
                    }
                },
                'slide-up': {
                    '0%': {
                        opacity: '0',
                        transform: 'translateY(10px)'
                    },
                    '100%': {
                        opacity: '1',
                        transform: 'translateY(0)'
                    }
                }
            },
            animation: {
                'fade-up-out': 'fade-up-out 0.2s ease-out forwards',
                'fade-in': 'fade-in 0.2s ease-out forwards',
                'slide-up': 'slide-up 0.2s ease-out'
            }
        }
    }
} 
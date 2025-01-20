# test-curves.py
#
# Python helper function to more easily visualize various possible smoothing
# functions for the peer distribution.

import numpy as np
import matplotlib.pyplot as plt

# import math


# def smooth_function(x, y_0, n, k=None):
#     return y_0 + (n - 5) / (1 + (math.e) ** (-x))

# Parameters
y_0 = 5  # Intercept above 0
n = 50  # Target value
k = 0.05  # Steepness
x = np.linspace(0, 200)  # x-values


def smooth_function(x, y_0, n, k):
    return y_0 + (n - y_0) * (np.tanh(k * x))


y = smooth_function(x, y_0, n, k)

# Plot the result
plt.plot(x, y)
plt.xlabel("x")
plt.ylabel("f(x)")
plt.title("Smoothed Function")
plt.grid(True)
plt.show()

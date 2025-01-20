# test-curves.py
#
# Python helper function to more easily visualize various possible smoothing
# functions for the peer distribution.
#
# There is no guarantee that the functions and constants here match
# algorithms.go

import numpy as np
import matplotlib.pyplot as plt


def smooth_function(x, numwant, good_seedcount):
    """
    Current function:
    y = y_0 + (numwant - y_0) * tanh(kx)
    k is the steepness, and it's calculated such that if a client is seeding
    more than 1 standard deviation above the average seed count, they get the
    maximum they requested.
    """

    # minimum peer count to distribute
    y_0 = 5

    # atanh(1) is infinite since the function is asymptotic, so we need to
    # calculate the steepness relative to a delta.
    delta = 0.1

    # add delta in the denominator to avoid division by zero when numwant == y_0.
    k = np.arctanh((numwant - y_0 - delta) / (numwant - y_0 + delta)) / good_seedcount

    return y_0 + (numwant - y_0) * (np.tanh(k * x))


if __name__ == "__main__":
    x = np.linspace(0, 200)  # x-values

    y = smooth_function(x, 50, 100)

    # Plot the result
    plt.plot(x, y)
    plt.xlabel("x")
    plt.ylabel("f(x)")
    plt.title("Smoothed Function")
    plt.grid(True)
    plt.show()

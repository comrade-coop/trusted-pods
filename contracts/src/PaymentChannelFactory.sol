// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.13;

import {IERC20} from "openzeppelin-contracts/contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "openzeppelin-contracts/contracts/token/ERC20/utils/SafeERC20.sol";

using SafeERC20 for IERC20;

contract PaymentChannelFactory {
    function channelAddress(bytes32 salt, bytes memory creationParameters) internal view returns (address) {
        return address(
            uint160(
                uint256(
                    keccak256(
                        abi.encodePacked(
                            bytes1(0xff),
                            address(this),
                            salt,
                            keccak256(abi.encodePacked(type(PaymentChannel).creationCode, creationParameters))
                        )
                    )
                )
            )
        );
    }

    function channelAddress(address publisher, address provider, IERC20 token, uint256 unlockTime, bytes32 salt)
        public
        view
        returns (address)
    {
        return channelAddress(salt, abi.encode(publisher, provider, token, unlockTime));
    }

    function createChannel(address provider, IERC20 token, uint256 unlockTime, bytes32 salt, uint256 initialAmount)
        public
        returns (PaymentChannel)
    {
        address publisher = msg.sender;

        PaymentChannel channel = new PaymentChannel{salt: salt}(publisher, provider, token, unlockTime);

        token.forceApprove(address(channel), type(uint256).max);

        token.safeTransferFrom(msg.sender, address(this), initialAmount);
        channel.deposit(initialAmount);

        return channel;
    }

    // add more funds to a payment channel created with createChannel (simplifies approve() calls)
    function deposit(PaymentChannel channel, uint256 amount) public {
        channel.token().safeTransferFrom(msg.sender, address(this), amount);
        channel.deposit(amount);
    }
}

contract PaymentChannel {
    error NoPermissions();
    error DoesNotExist();
    error AmountRequired();
    error ChannelLocked();
    error InsufficientFunds();

    event UnlockTimerStarted(uint256 unlockedAt);
    event UnlockTimerStopped();
    event Withdrawn(uint256 withdrawnAmount);

    uint256 public investedByPublisher;
    uint256 public withdrawnByProvider;
    uint256 public immutable unlockTime; // minimum time in seconds needed to unlock the funds
    uint256 public unlockedAt; // time @ unlock + unlockTime

    address internal immutable factory;
    address public immutable publisher;
    address public immutable provider;
    IERC20 public immutable token;

    // called by publisher to create a new payment channel; must approve a withdraw by this contract's address
    constructor(address _publisher, address _provider, IERC20 _token, uint256 _unlockTime) {
        if (publisher == address(0)) publisher = msg.sender;
        factory = msg.sender;
        publisher = _publisher;
        provider = _provider;
        token = _token;
        unlockTime = _unlockTime;
    }

    modifier requireSender(address expectedSender) {
        if (msg.sender != expectedSender) revert NoPermissions();
        _;
    }

    // add more funds to the payment channel
    function deposit(uint256 amount) public {
        // requireSender(publisher, factory)
        if (amount == 0) revert AmountRequired();

        investedByPublisher = investedByPublisher + amount;

        token.safeTransferFrom(msg.sender, address(this), amount);
    }

    // initiate the process of unlocking the funds stored in the contract
    function unlock() public requireSender(publisher) {
        uint256 newUnlockedAt = block.timestamp + unlockTime;
        if (unlockedAt == 0 || unlockedAt < newUnlockedAt) {
            unlockedAt = newUnlockedAt;
        }

        emit UnlockTimerStarted(unlockedAt);
    }

    // stop the process of unlocking the funds stored in the contract
    function lock() public requireSender(publisher) {
        if (unlockedAt == 0) revert ChannelLocked();
        unlockedAt = 0;

        emit UnlockTimerStopped();
    }

    // transfer the now-unlocked funds back to the publisher
    function withdrawUnlocked() public requireSender(publisher) {
        if (unlockedAt == 0 || block.timestamp < unlockedAt) revert ChannelLocked();

        uint256 leftoverFunds = investedByPublisher - withdrawnByProvider;
        investedByPublisher = withdrawnByProvider;

        if (leftoverFunds == 0) revert AmountRequired();

        token.safeTransfer(publisher, leftoverFunds);
    }

    // allows the provider to withdraw as many tokens as would be needed to reach totalWithdrawlAmount since the start of the channel
    function withdrawUpTo(uint256 totalAmount, address transferAddress) public requireSender(provider) {
        if (transferAddress == address(0)) {
            transferAddress = msg.sender;
        }

        if (totalAmount > investedByPublisher) revert InsufficientFunds();
        if (totalAmount <= withdrawnByProvider) revert AmountRequired();

        uint256 transferAmonut = totalAmount - withdrawnByProvider;
        withdrawnByProvider = totalAmount;
        emit Withdrawn(transferAmonut);

        if (unlockedAt != 0) {
            unlockedAt = block.timestamp;
        }

        token.safeTransfer(transferAddress, transferAmonut);
    }

    // allows the provider to withdraw amount more tokens
    function withdraw(uint256 amount, address transferAddress) public requireSender(provider) {
        withdrawUpTo(withdrawnByProvider + amount, transferAddress);
    }

    // allows one to check the amount of as-of-yet unclaimed tokens
    function available() public view returns (uint256) {
        return investedByPublisher - withdrawnByProvider;
    }
}
